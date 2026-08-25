import AxeBuilder from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'
import { randomUUID } from 'node:crypto'

interface VehicleRecord { id: string; name: string }
interface Enrollment { token: string; server_url: string; server_version: string }
interface EnrolledDevice { device_id: string; credential: string; config: { sampling: { default_seconds: number; parked_seconds: number }; upload: { default_seconds: number; parked_seconds: number } } }
interface HookRecord { id: string }
interface HookExecution { status: string; logs: Array<Record<string, unknown>> }

const ONE_PIXEL_PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2ZQAAAABJRU5ErkJggg==',
  'base64',
)

async function csrfToken(page: Page): Promise<string> {
  const cookies = await page.context().cookies()
  const csrf = cookies.find((cookie) => cookie.name === 'vehinode_csrf')?.value
  if (!csrf) throw new Error('browser session has no CSRF cookie')
  return csrf
}

async function browserJson<T>(page: Page, method: 'get' | 'post' | 'put', path: string, data?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (method !== 'get') headers['X-CSRF-Token'] = await csrfToken(page)
  const response = await page.request[method](path, { data, headers })
  expect(response.ok(), `${method.toUpperCase()} ${path}: ${await response.text()}`).toBeTruthy()
  return response.json() as Promise<T>
}

async function waitForExecutions(page: Page, hookId: string, count: number): Promise<HookExecution[]> {
  let rows: HookExecution[] = []
  await expect.poll(async () => {
    rows = await browserJson<HookExecution[]>(page, 'get', `/api/v1/hooks/${hookId}/executions`)
    return rows.filter((row) => row.status === 'success').length
  }, { timeout: 15_000 }).toBe(count)
  return rows
}

test('complete browser journey from bootstrapped admin to persistent hook state', async ({ page, request }) => {
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: 'Sign in to VehiNode' })).toBeVisible()
  await expect(page.getByText('Open-source software · operated by you')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Create the initial administrator' })).toHaveCount(0)
  const loginAccessibility = await new AxeBuilder({ page }).analyze()
  expect(loginAccessibility.violations).toEqual([])

  await page.getByLabel('Email').fill('browser-owner@example.com')
  await page.getByLabel('Password').fill('browser-e2e-password-2026')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL('/')
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
  await expect(page.getByRole('button', { name: /Overview/ })).toBeVisible()

  const rejectedRegistration = await request.post('/api/v1/auth/register', {
    data: {
      email: 'second-owner@example.com',
      password: 'another-browser-password-2026',
      display_name: 'Second Owner',
    },
  })
  expect(rejectedRegistration.status()).toBe(403)

  await page.getByRole('link', { name: 'Vehicles', exact: true }).click()
  await page.getByRole('button', { name: 'Add vehicle' }).click()
  await page.getByLabel('Name', { exact:true }).fill('Éclair')
  await page.getByRole('button', { name: 'Create vehicle' }).click()
  await expect(page.getByRole('heading', { name: 'Éclair' })).toBeVisible()
  await expect(page.getByRole('img', { name: 'No photo for Éclair' })).toBeVisible()
  await expect(page.locator('.vehicle-photo-placeholder .app-icon')).toBeVisible()
  const emptyBatteryCenterOffset = await page.locator('.charge-reading').evaluate((section) => {
    const value = section.querySelector('strong')!.getBoundingClientRect()
    const bounds = section.getBoundingClientRect()
    return Math.abs((value.left + value.width / 2) - (bounds.left + bounds.width / 2))
  })
  expect(emptyBatteryCenterOffset).toBeLessThan(1)
  const photoInput = page.locator('.vehicle-media input[type="file"]')
  await photoInput.focus()
  await expect(photoInput.locator('..')).toHaveCSS('outline-style', 'solid')
  const vehicleAccessibility = await new AxeBuilder({ page }).analyze()
  expect(vehicleAccessibility.violations).toEqual([])
  await page.evaluate(() => { document.documentElement.dataset.theme = 'dark' })
  await expect.poll(() => page.getByRole('heading', { name: 'Vehicles', exact: true }).evaluate((element) => getComputedStyle(element).color)).toBe('rgb(241, 243, 239)')
  await expect.poll(() => page.locator('html').evaluate((element) => getComputedStyle(element).filter)).toBe('none')
  const darkVehicleAccessibility = await new AxeBuilder({ page }).analyze()
  expect(darkVehicleAccessibility.violations).toEqual([])
  await page.evaluate(() => { document.documentElement.dataset.theme = 'light' })
  const cardHeightWithoutPhoto = await page.locator('.vehicle-card', { hasText:'Éclair' }).evaluate((card) => card.getBoundingClientRect().height)
  await photoInput.setInputFiles({
    name: 'eclair.png',
    mimeType: 'image/png',
    buffer: ONE_PIXEL_PNG,
  })
  await expect(page.getByText('Photo saved for Éclair.')).toBeVisible()
  const vehiclePhoto = page.locator('.vehicle-media img')
  await expect(vehiclePhoto).toBeVisible()
  expect(await vehiclePhoto.evaluate((image: HTMLImageElement) => image.naturalWidth)).toBe(1)
  const cardHeightWithPhoto = await page.locator('.vehicle-card', { hasText:'Éclair' }).evaluate((card) => card.getBoundingClientRect().height)
  expect(Math.abs(cardHeightWithPhoto - cardHeightWithoutPhoto)).toBeLessThan(1)
  const [vehicle] = await browserJson<VehicleRecord[]>(page, 'get', '/api/v1/vehicles')
  expect(vehicle?.name).toBe('Éclair')
  await browserJson<VehicleRecord>(page, 'post', '/api/v1/vehicles', {
    name: 'Touring', vehicle_profile: null,
  })
  await page.reload()
  const secondVehicleCard = page.locator('.vehicle-card', { hasText: 'Touring' })
  await expect(secondVehicleCard).toBeVisible()
  await expect(secondVehicleCard.getByText(/Electric|Hybrid|Petrol|Diesel/)).toHaveCount(0)

  await page.locator('.sidebar').getByRole('link', { name:'Profiles', exact:true }).click()
  await expect(page.getByRole('heading', { name:'Telemetry profiles' })).toBeVisible()
  await page.getByRole('button', { name:'New profile' }).click()
  await expect(page.getByRole('dialog', { name:'Create profile' })).toBeVisible()
  await page.getByRole('dialog', { name:'Create profile' }).getByRole('button', { name:'Add signal' }).click()
  await expect(page.getByRole('dialog', { name:'Add signal' })).toBeVisible()
  await page.getByRole('dialog', { name:'Add signal' }).getByRole('button', { name:'Close' }).click()
  await page.getByRole('dialog', { name:'Create profile' }).getByRole('button', { name:'Close' }).click()

  await page.getByRole('link', { name: 'Devices' }).click()
  await page.getByRole('button', { name: 'Add tracker' }).click()
  const enrollmentDialog = page.getByRole('dialog', { name:'Enroll a tracker' })
  const implementationSelect = enrollmentDialog.getByLabel('Agent implementation')
  await expect(implementationSelect).toContainText('VehiNode Go agent')
  await expect(enrollmentDialog.locator('.implementation-picker')).toContainText('Raspberry Pi')
  const presetSelect = enrollmentDialog.getByLabel('Preset')
  await expect(presetSelect).toContainText('Data saver')
  await presetSelect.focus()
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Enter')
  await expect(presetSelect).toContainText('Balanced')
  await expect(enrollmentDialog.getByLabel('Driving interval (seconds)')).toHaveValue('30')
  await expect(enrollmentDialog.getByLabel('Parked interval (seconds)')).toHaveValue('900')
  await page.setViewportSize({ width:390, height:844 })
  const enrollmentDimensions = await enrollmentDialog.evaluate((dialog) => ({ scroll:dialog.scrollWidth, width:dialog.clientWidth }))
  expect(enrollmentDimensions.scroll).toBeLessThanOrEqual(enrollmentDimensions.width)
  const enrollmentAccessibility = await new AxeBuilder({ page }).include('.app-modal').analyze()
  expect(enrollmentAccessibility.violations).toEqual([])
  await page.setViewportSize({ width:1440, height:1000 })
  const enrollmentResponse = page.waitForResponse((response) => response.url().includes('/enrollments') && response.request().method() === 'POST')
  await page.locator('.enrollment-panel').getByRole('button', { name: 'Create enrollment' }).click()
  const enrollment = await (await enrollmentResponse).json() as Enrollment
  const readyDialog = page.getByRole('dialog', { name:'Enrollment ready' })
  await expect(readyDialog.locator('.setup-steps li')).toHaveCount(1)
  await expect(readyDialog.locator('pre')).toContainText('--token')
  await expect(readyDialog).toContainText('Setup for VehiNode Go agent')
  await page.setViewportSize({ width:390, height:844 })
  const readyDimensions = await readyDialog.evaluate((dialog) => ({ scroll:dialog.scrollWidth, width:dialog.clientWidth }))
  expect(readyDimensions.scroll).toBeLessThanOrEqual(readyDimensions.width)
  const readyAccessibility = await new AxeBuilder({ page }).include('.app-modal').analyze()
  expect(readyAccessibility.violations).toEqual([])
  await page.setViewportSize({ width:1440, height:1000 })
  const copyCommand = page.getByRole('button', { name: 'Copy command' })
  await expect(copyCommand).toBeVisible()
  await expect(copyCommand).toHaveText('')
  await readyDialog.getByRole('button', { name:'Done' }).click()

  const enrolledResponse = await request.post('/api/v1/device/enroll', {
    data: { token: enrollment.token, agent_version: '0.2.0' },
  })
  expect(enrolledResponse.status()).toBe(201)
  const enrolled = await enrolledResponse.json() as EnrolledDevice
  expect(enrolled.config.sampling.default_seconds).toBe(30)
  expect(enrolled.config.sampling.parked_seconds).toBe(900)
  expect(enrolled.config.upload.default_seconds).toBe(30)
  expect(enrolled.config.upload.parked_seconds).toBe(900)
  const isolatedHumanRequest = await request.get('/api/v1/auth/me', { headers: { Authorization: `Device ${enrolled.credential}` } })
  expect(isolatedHumanRequest.status()).toBe(401)

  const samples = Array.from({ length: 6 }, (_, index) => ({
    id: randomUUID(),
    sequence: index,
    recorded_at: new Date(Date.now() - (5 - index) * 5_000).toISOString(),
    position: { latitude: 48.8566 + index * 0.002, longitude: 2.3522 + index * 0.003, speed: 24 + index * 7, heading: 35 + index * 8, altitude: 42, accuracy: 4.5 },
    metrics: { 'battery.soc': 82 - index, 'battery.pack_voltage': 330.5, 'battery.power': -11.8, 'charging.active': false, 'vehicle.speed': 24 + index * 7 },
    device: { mobile_signal: -76, queue_depth: 0 },
  }))
  const bootId = randomUUID()
  const firstBatch = await request.post('/api/v1/device/telemetry/batch', {
    headers: { Authorization: `Device ${enrolled.credential}` },
    data: { boot_id: bootId, samples },
  })
  expect(firstBatch.status()).toBe(200)
  expect((await firstBatch.json()).accepted).toHaveLength(6)
  const retriedBatch = await request.post('/api/v1/device/telemetry/batch', {
    headers: { Authorization: `Device ${enrolled.credential}` },
    data: { boot_id: bootId, samples },
  })
  expect(retriedBatch.status()).toBe(200)
  expect((await retriedBatch.json()).duplicates).toHaveLength(6)

  await page.reload()
  await expect(page.locator('.version-pill.warning')).toHaveText('Minor differs')

  await page.locator('.sidebar').getByRole('link', { name: 'Dashboards', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
  await expect(page.locator('[data-widget-type="battery-gauge"] .energy-value')).toHaveText('77')
  await expect(page.locator('.vehinode-position-marker')).toBeVisible()
  const initialDashboardAccessibility = await new AxeBuilder({ page }).analyze()
  expect(initialDashboardAccessibility.violations).toEqual([])

  const vehicleSelector = page.locator('[data-widget-type="vehicle-selector"]')
  await expect(vehicleSelector.getByRole('combobox')).toHaveCount(1)
  await vehicleSelector.getByRole('combobox').click()
  await page.getByPlaceholder('Search vehicles…').fill('Touring')
  await expect(page.getByRole('option')).toHaveCount(1)
  await page.getByRole('option', { name:'Touring', exact:true }).click()
  await expect(page.locator('[data-widget-type="battery-gauge"] .dashboard-widget-empty')).toContainText('No data yet')
  await expect(page.locator('[data-widget-type="position-map"] .vehicle-map')).toHaveCount(0)
  await vehicleSelector.getByRole('combobox').click()
  await page.getByPlaceholder('Search vehicles…').fill('Éclair')
  await page.getByRole('option', { name:/Éclair/ }).click()
  await expect(page.locator('[data-widget-type="battery-gauge"] .energy-value')).toHaveText('77')

  const liveSample = {
    id: randomUUID(),
    sequence: 6,
    recorded_at: new Date().toISOString(),
    position: { latitude: 48.87, longitude: 2.37, speed: 18, heading: 102, altitude: 43, accuracy: 3.8 },
    metrics: { 'battery.soc': 61, 'battery.pack_voltage': 329.1, 'battery.power': -4.2, 'charging.active': false, 'vehicle.speed': 18 },
    device: { mobile_signal: -73, queue_depth: 0 },
  }
  const liveBatch = await request.post('/api/v1/device/telemetry/batch', {
    headers: { Authorization: `Device ${enrolled.credential}` },
    data: { boot_id: bootId, samples: [liveSample] },
  })
  expect(liveBatch.status()).toBe(200)
  await page.reload()
  await expect(page.locator('[data-widget-type="battery-gauge"] .energy-value')).toHaveText('61')

  const dashboardAccessibility = await new AxeBuilder({ page }).analyze()
  expect(dashboardAccessibility.violations).toEqual([])
  await page.getByTitle('Theme').click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect.poll(() => page.getByRole('heading', { name: 'Overview' }).evaluate((element) => getComputedStyle(element).color)).toBe('rgb(241, 243, 239)')
  const darkDashboardAccessibility = await new AxeBuilder({ page }).analyze()
  expect(darkDashboardAccessibility.violations).toEqual([])
  await page.getByTitle('Theme').click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')

  await page.getByRole('link', { name: 'Vehicles', exact: true }).click()
  await page.locator('.vehicle-card', { hasText:'Éclair' }).getByRole('link', { name:'History' }).click()
  await expect(page.getByRole('heading', { name: 'Éclair · History' })).toBeVisible()
  await expect(page.locator('.history-stat', { hasText:'Source samples' })).toContainText('7')
  await expect(page.locator('.route-count')).toContainText('7')
  expect(await page.locator('select').count()).toBe(0)
  await page.getByRole('combobox', { name:'Metric' }).click()
  const metricMenu = page.locator('.app-select-menu')
  await expect(metricMenu).toBeVisible()
  const menuBounds = await metricMenu.boundingBox()
  expect(menuBounds).not.toBeNull()
  expect(menuBounds!.x + menuBounds!.width).toBeLessThanOrEqual(await page.evaluate(() => window.innerWidth))
  await page.keyboard.press('Escape')

  await page.locator('.sidebar').getByRole('link', { name: 'Dashboards', exact: true }).click()
  await page.getByRole('button', { name: 'Dashboard actions' }).click()
  await page.getByRole('menuitem', { name: 'New dashboard' }).click()
  await page.locator('.app-modal').getByLabel('Dashboard name').fill('Diagnostics')
  await page.getByRole('button', { name: 'Create dashboard' }).click()
  await page.getByRole('button', { name: 'Add widget' }).click()
  await page.getByLabel('Title').fill('Battery at a glance')
  await page.locator('.app-modal').getByRole('button', { name: 'Add widget' }).click()
  await expect(page.getByText('Battery at a glance')).toBeVisible()
  await page.getByRole('button', { name: 'Make default' }).click()
  await page.getByRole('button', { name: 'Save', exact:true }).click()
  await expect(page.getByText('Dashboard saved.')).toBeVisible()
  await expect(page.getByRole('button', { name:'Dashboard actions' })).toBeVisible()
  await expect(page.locator('.widget-remove')).toHaveCount(0)
  await page.reload()
  await expect(page.getByText('Battery at a glance')).toBeVisible()
  await expect(page.getByRole('button', { name: /Diagnostics.*Default/ })).toBeVisible()

  const hooksLoaded = page.waitForResponse((response) => response.url().endsWith('/api/v1/hooks') && response.request().method() === 'GET')
  await page.getByRole('link', { name: 'Hooks' }).click()
  await hooksLoaded
  await page.locator('.page-header').getByRole('button', { name: 'New hook' }).click()
  const hookModal = page.locator('.app-modal')
  await hookModal.getByLabel('Name', { exact:true }).fill('Browser state counter')
  await hookModal.getByLabel('Description').fill('Verifies persistent hook state through the browser flow')
  await hookModal.locator('.cm-content').fill('ctx.state["runs"] = ctx.state.get("runs", 0) + 1\nctx.log.info("browser e2e", runs=ctx.state["runs"], dry_run=ctx.dry_run)')
  await hookModal.getByRole('button', { name: 'Save', exact: true }).click()
  const [hook] = await browserJson<HookRecord[]>(page, 'get', '/api/v1/hooks')
  expect(hook?.id).toBeTruthy()

  await page.getByRole('button', { name: 'Test with telemetry' }).click()
  await waitForExecutions(page, hook.id, 1)
  await page.getByRole('button', { name: 'Test with telemetry' }).click()
  const executions = await waitForExecutions(page, hook.id, 2)
  expect(JSON.stringify(executions[0]?.logs)).toContain('"runs":2')
  await page.reload()
  await page.locator('.hook-list').getByRole('button', { name: /Browser state counter/ }).click()
  await expect(page.getByText('success').first()).toBeVisible()

  await page.getByRole('link', { name: 'Devices' }).click()
  await expect(page.getByRole('heading', { name: 'Vehicle tracker' })).toBeVisible()
  await expect(page.getByText('0.2.0')).toBeVisible()
  const trackerRow = page.locator('.device-row', { hasText:'Vehicle tracker' })
  await trackerRow.getByRole('button', { name:'Configure' }).click()
  const configurationDialog = page.getByRole('dialog', { name:'Configure Vehicle tracker' })
  await expect(configurationDialog.getByLabel('Driving interval (seconds)')).toHaveValue('30')
  await expect(configurationDialog.getByLabel('Parked interval (seconds)')).toHaveValue('900')
  const configurationAccessibility = await new AxeBuilder({ page }).include('.app-modal').analyze()
  expect(configurationAccessibility.violations).toEqual([])
  await configurationDialog.getByLabel('Preset').click()
  await page.getByRole('option', { name:'Light live' }).click()
  const configurationResponse = page.waitForResponse((response) => response.url().includes('/telemetry-policy') && response.request().method() === 'PUT')
  await configurationDialog.getByRole('button', { name:'Save', exact:true }).click()
  await configurationResponse
  await expect(page.getByText('Telemetry policy saved for Vehicle tracker.')).toBeVisible()
  await expect(trackerRow).toContainText('Driving: 5 s; parked: 15 min')

  await page.setViewportSize({ width: 375, height: 812 })
  await page.locator('.sidebar').getByRole('link', { name: 'Dashboards', exact: true }).click()
  await page.getByRole('button', { name: /Overview/ }).click()
  await expect(page.locator('[data-widget-type="battery-gauge"] .energy-widget')).toBeVisible()
  const badgeGeometry = await page.locator('.status').evaluateAll((badges) => badges.map((badge) => {
    const bounds = badge.getBoundingClientRect()
    return { radius: getComputedStyle(badge).borderRadius, width: bounds.width, height: bounds.height }
  }))
  expect(badgeGeometry.length).toBeGreaterThan(0)
  expect(badgeGeometry.every((badge) => badge.radius === '6px' && badge.width > badge.height)).toBeTruthy()
  const mobileDimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, viewport: window.innerWidth }))
  expect(mobileDimensions.scroll).toBeLessThanOrEqual(mobileDimensions.viewport)

  await page.getByRole('link', { name: 'Vehicles', exact: true }).click()
  await expect(page.locator('.vehicle-card').first()).toBeVisible()
  await expect(page.locator('.vehicle-card img')).toBeVisible()
  const mobileGarageDimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, viewport: window.innerWidth }))
  expect(mobileGarageDimensions.scroll).toBeLessThanOrEqual(mobileGarageDimensions.viewport)
})

test('mobile login keeps language, theme, keyboard access and reflow', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  await page.emulateMedia({ colorScheme: 'light', reducedMotion: 'reduce' })
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'languages', { configurable:true, get:() => ['fr-FR', 'en-US'] })
  })
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: 'Connectez-vous à VehiNode' })).toBeVisible()

  await page.keyboard.press('Tab')
  const language = page.locator('.login-utilities [role="combobox"]')
  await expect(language).toBeFocused()
  await language.click()
  await page.getByRole('option', { name: 'EN' }).click()
  await language.click()
  await page.getByRole('option', { name: 'FR' }).click()
  await expect(page.getByRole('heading', { name: 'Connectez-vous à VehiNode' })).toBeVisible()
  await page.getByTitle('Thème').click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, viewport: window.innerWidth }))
  expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.viewport)
  const accessibility = await new AxeBuilder({ page }).analyze()
  expect(accessibility.violations).toEqual([])
})
