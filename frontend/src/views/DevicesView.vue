<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api/client'
import type { AgentImplementation, AgentSetupStep, Device, DeviceEnrollment, TelemetryPolicy, Vehicle } from '../api/types'
import AppIcon from '../components/AppIcon.vue'
import AppModal from '../components/AppModal.vue'
import AppSelect from '../components/AppSelect.vue'
import TelemetryPolicyPicker from '../components/TelemetryPolicyPicker.vue'

const DATA_SAVER_POLICY: TelemetryPolicy = { sampling_seconds:120, upload_seconds:120, parked_sampling_seconds:900, parked_upload_seconds:900 }
const { t } = useI18n()
const devices = ref<Device[]>([])
const vehicles = ref<Vehicle[]>([])
const agentImplementations = ref<AgentImplementation[]>([])
const dialogMode = ref<'enroll'|'configure'|null>(null)
const selectedVehicle = ref('')
const trackerName = ref('Vehicle tracker')
const enrollmentPolicy = ref<TelemetryPolicy>({ ...DATA_SAVER_POLICY })
const enrollment = ref<DeviceEnrollment|null>(null)
const selectedImplementation = ref('')
const creating = ref(false)
const enrollmentError = ref('')
const copied = ref(false)
const rotatedCredential = ref<{id:string;credential:string}|null>(null)
const editingDevice = ref<Device|null>(null)
const editingPolicy = ref<TelemetryPolicy>({ ...DATA_SAVER_POLICY })
const savingPolicy = ref(false)
const policyError = ref('')
const policyStatus = ref('')
const error = ref('')
const vehicleNames = computed(() => Object.fromEntries(vehicles.value.map((item) => [item.id, item.name])))
const enrollmentPolicyValid = computed(() => validPolicy(enrollmentPolicy.value))
const editingPolicyValid = computed(() => validPolicy(editingPolicy.value))
const selectedInstallation = computed(() => enrollment.value?.implementations.find((item) => item.id === selectedImplementation.value))
const selectedCatalogAgent = computed(() => agentImplementations.value.find((item) => item.id === selectedImplementation.value))
const setupSteps = computed<AgentSetupStep[]>(() => selectedInstallation.value?.setup_steps ?? [])
const apiDocsUrl = computed(() => enrollment.value ? `${enrollment.value.server_url}/api/docs` : '')
const dialogTitle = computed(() => {
  if (dialogMode.value === 'configure') return t('devices.configureTitle', { name:editingDevice.value?.name ?? '' })
  return enrollment.value ? t('devices.enrollmentReady') : t('devices.enrollTitle')
})

async function load() {
  try {
    ;[devices.value, vehicles.value, agentImplementations.value] = await Promise.all([
      api<Device[]>('/devices'),
      api<Vehicle[]>('/vehicles'),
      api<AgentImplementation[]>('/agent-implementations'),
    ])
    if (!selectedVehicle.value && vehicles.value[0]) selectedVehicle.value = vehicles.value[0].id
  }
  catch (reason) { error.value = reason instanceof Error ? reason.message : t('common.error') }
}
function openEnrollment() {
  enrollment.value = null
  selectedImplementation.value = agentImplementations.value[0]?.id ?? 'custom'
  copied.value = false
  enrollmentError.value = ''
  enrollmentPolicy.value = { ...DATA_SAVER_POLICY }
  if (!vehicles.value.some((vehicle) => vehicle.id === selectedVehicle.value)) {
    selectedVehicle.value = vehicles.value[0]?.id ?? ''
  }
  dialogMode.value = 'enroll'
}
function closeTrackerDialog():void { dialogMode.value=null;editingDevice.value=null }
async function createEnrollment() {
  if (!selectedVehicle.value || !selectedImplementation.value || !enrollmentPolicyValid.value || creating.value) return
  creating.value = true
  enrollmentError.value = ''
  try {
    const response = await api<DeviceEnrollment>(`/vehicles/${selectedVehicle.value}/enrollments`, { method:'POST', body:JSON.stringify({ name:trackerName.value, telemetry_policy:enrollmentPolicy.value }) })
    enrollment.value = response
  } catch (reason) {
    enrollmentError.value = reason instanceof Error ? reason.message : t('common.error')
  } finally { creating.value = false }
}
async function copy(value:string) { if (!value) return; await navigator.clipboard.writeText(value); copied.value = true; window.setTimeout(() => copied.value = false, 1500) }
async function revoke(id:string) { if (!confirm(t('devices.revoke') + '?')) return; await api(`/devices/${id}/revoke`, { method:'POST' }); await load() }
async function rotate(id:string) { const response = await api<{credential:string}>(`/devices/${id}/rotate`, {method:'POST'}); rotatedCredential.value={id,credential:response.credential} }
async function copyCredential() { if(!rotatedCredential.value)return;await navigator.clipboard.writeText(rotatedCredential.value.credential);copied.value=true;window.setTimeout(()=>copied.value=false,1500) }
function stepText(step:AgentSetupStep):string { return step.text || t(`devices.stepDefaults.${step.kind}`) }
function validPolicy(policy:TelemetryPolicy):boolean { return policy.sampling_seconds >= 1 && policy.sampling_seconds <= 86400 && policy.upload_seconds === policy.sampling_seconds && policy.parked_sampling_seconds >= 1 && policy.parked_sampling_seconds <= 86400 && policy.parked_upload_seconds === policy.parked_sampling_seconds }
function formatInterval(seconds:number):string { if (seconds % 3600 === 0) return `${seconds / 3600} h`; if (seconds % 60 === 0) return `${seconds / 60} min`; return `${seconds} s` }
function policySummary(policy:TelemetryPolicy|undefined):string { const value=policy??DATA_SAVER_POLICY;return t('devices.policySummary',{moving:formatInterval(value.sampling_seconds),parked:formatInterval(value.parked_sampling_seconds)}) }
function openPolicy(device:Device):void { editingDevice.value=device;editingPolicy.value={...(device.telemetry_policy??DATA_SAVER_POLICY)};policyError.value='';dialogMode.value='configure' }
async function savePolicy():Promise<void> {
  if (!editingDevice.value || !editingPolicyValid.value || savingPolicy.value) return
  savingPolicy.value=true;policyError.value=''
  try {
    await api<TelemetryPolicy>(`/devices/${editingDevice.value.id}/telemetry-policy`,{method:'PUT',body:JSON.stringify(editingPolicy.value)})
    policyStatus.value=t('devices.policySaved',{name:editingDevice.value.name})
    closeTrackerDialog()
    await load()
  } catch(reason) { policyError.value=reason instanceof Error?reason.message:t('common.error') }
  finally { savingPolicy.value=false }
}
const onlineCount = computed(() => devices.value.filter((device) => device.online && !device.revoked_at).length)
const versionCount = computed(() => new Set(devices.value.flatMap((device) => device.agent_version ? [device.agent_version] : [])).size)
onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-header"><div><span class="eyebrow">{{ t('devices.eyebrow') }}</span><h1>{{ t('devices.title') }}</h1><p>{{ t('devices.summary',{count:devices.length,online:onlineCount,versions:versionCount}) }}</p></div><button class="button" :disabled="!vehicles.length" @click="openEnrollment"><AppIcon name="plus" :size="15" />{{ t('devices.add') }}</button></header>
    <p v-if="error" class="error">{{ error }}</p><p v-if="policyStatus" class="save-status" role="status">{{ policyStatus }}</p>
    <AppModal :open="dialogMode!==null" :title="dialogTitle" wide @close="closeTrackerDialog">
      <template #eyebrow><span class="eyebrow">{{ dialogMode==='configure' ? t('devices.policyTitle') : t('devices.add') }}</span></template>
      <form v-if="dialogMode==='enroll'" class="enrollment-panel" @submit.prevent="createEnrollment">
        <div v-if="!enrollment" class="enrollment-fields">
          <label class="field"><span>{{ t('devices.vehicle') }}</span><AppSelect v-model="selectedVehicle" searchable :search-placeholder="t('vehicles.search')" :no-results-text="t('vehicles.noMatch')"><option v-for="vehicle in vehicles" :key="vehicle.id" :value="vehicle.id">{{ vehicle.name }}</option></AppSelect></label>
          <label class="field"><span>{{ t('devices.name') }}</span><input v-model="trackerName" class="input" required maxlength="120" /></label>
          <label class="field implementation-picker">
            <span>{{ t('devices.implementationTitle') }}</span>
            <AppSelect v-model="selectedImplementation" :aria-label="t('devices.implementationTitle')">
              <option v-for="implementation in agentImplementations" :key="implementation.id" :value="implementation.id">{{ implementation.name }}</option>
              <option value="custom">{{ t('devices.customAgent') }}</option>
            </AppSelect>
            <small>{{ t('devices.implementationHint') }}</small>
            <small v-if="selectedCatalogAgent" class="agent-facts">{{ selectedCatalogAgent.hardware }} · {{ t(`devices.setupKind.${selectedCatalogAgent.setup_kind}`) }}</small>
          </label>
          <TelemetryPolicyPicker v-model="enrollmentPolicy" />
        </div>
        <p v-if="enrollmentError" class="error" role="alert">{{ enrollmentError }}</p>
        <button v-if="!enrollment" class="button enrollment-submit" :disabled="!selectedVehicle||!selectedImplementation||!enrollmentPolicyValid||creating">{{ creating ? t('devices.creating') : t('devices.createEnrollment') }}</button>
        <div v-else class="command-reveal">
          <p class="reveal-context">{{ t('devices.setupFor',{name:selectedImplementation==='custom' ? t('devices.customAgent') : selectedInstallation?.name??selectedImplementation}) }}</p>
          <ol v-if="selectedImplementation!=='custom'" class="setup-steps">
            <li v-for="(step,index) in setupSteps" :key="index">
              <p class="step-text">{{ stepText(step) }}</p>
              <div v-if="step.kind==='command'" class="copy-surface"><pre class="mono" tabindex="0" :aria-label="t('devices.installCommand')">{{ step.command }}</pre><button class="copy-button" type="button" :title="t('devices.copy')" :aria-label="t('devices.copy')" @click="copy(step.command)"><AppIcon :name="copied ? 'check' : 'copy'" :size="17" /></button></div>
              <p v-else-if="step.kind==='value'" class="step-value"><code class="mono">{{ step.value }}</code><button class="inline-copy" type="button" :aria-label="t('devices.copyValue')" @click="copy(step.value)"><AppIcon name="copy" :size="15" /></button></p>
              <p v-else-if="step.kind==='link'" class="step-link"><a :href="step.url" target="_blank" rel="noreferrer">{{ t('devices.openStep') }}</a></p>
            </li>
          </ol>
          <p v-if="selectedImplementation!=='custom'&&selectedInstallation?.docs_url" class="setup-docs"><a :href="selectedInstallation.docs_url" target="_blank" rel="noreferrer">{{ t('devices.agentDocs',{name:selectedInstallation.name}) }}</a></p>
          <div v-if="selectedImplementation==='custom'" class="custom-connection">
            <p>{{ t('devices.customDetailsHint') }}</p>
            <dl class="connection-facts">
              <div><dt>{{ t('devices.serverUrl') }}</dt><dd><code class="mono">{{ enrollment.server_url }}</code><button class="inline-copy" type="button" :aria-label="t('devices.copyServer')" @click="copy(enrollment.server_url)"><AppIcon name="copy" :size="15" /></button></dd></div>
              <div><dt>{{ t('devices.enrollmentToken') }}</dt><dd><code class="mono">{{ enrollment.token }}</code><button class="inline-copy" type="button" :aria-label="t('devices.copyToken')" @click="copy(enrollment.token)"><AppIcon name="copy" :size="15" /></button></dd></div>
              <div><dt>{{ t('devices.enrollmentEndpoint') }}</dt><dd><code class="mono">POST /api/v1/device/enroll</code></dd></div>
              <div><dt>{{ t('devices.minimumPayload') }}</dt><dd><code class="mono">token + agent_version</code><small>{{ t('devices.optionalFields') }}</small></dd></div>
            </dl>
            <p class="custom-footer"><a :href="apiDocsUrl" target="_blank" rel="noreferrer">{{ t('devices.apiReference') }}</a><span>{{ t('devices.serverVersion',{version:enrollment.server_version}) }}</span></p>
          </div>
          <span v-if="copied" class="copy-feedback" role="status">{{ t('devices.copied') }}</span>
          <button class="button enrollment-done" type="button" @click="closeTrackerDialog">{{ t('devices.done') }}</button>
        </div>
      </form>
      <form v-else-if="dialogMode==='configure'" class="policy-form" @submit.prevent="savePolicy">
        <TelemetryPolicyPicker v-model="editingPolicy" />
        <p v-if="policyError" class="error" role="alert">{{ policyError }}</p>
        <div class="form-actions"><button class="button" :disabled="!editingPolicyValid||savingPolicy">{{ savingPolicy?t('devices.saving'):t('common.save') }}</button><button class="button secondary" type="button" @click="closeTrackerDialog">{{ t('common.cancel') }}</button></div>
      </form>
    </AppModal>
    <section class="panel device-roster">
      <header class="roster-heading"><h2>{{ t('devices.roster') }}</h2><span>{{ devices.length }}</span></header>
      <article v-for="device in devices" :key="device.id" class="device-row">
        <div class="device-identity"><span class="device-icon"><AppIcon name="devices" /></span><div><h2>{{ device.name }}</h2><p>{{ vehicleNames[device.vehicle_id] }}</p></div><span :class="['status',{online:device.online&&!device.revoked_at}]">{{ device.revoked_at ? t('devices.revoked') : device.online ? t('common.online') : device.last_seen_at ? t('common.stale') : t('common.never') }}</span></div>
        <dl><div><dt>{{ t('devices.version') }}</dt><dd class="version-value"><span class="mono">{{ device.agent_version ?? '—' }}</span><span v-if="device.version_compatibility==='warning'" class="version-pill warning">{{ t('devices.versionDifference') }}</span><span v-else-if="device.version_compatibility==='incompatible'" class="version-pill incompatible">{{ t('devices.versionIncompatible') }}</span><span v-else-if="device.version_compatibility==='unknown'" class="version-pill warning">{{ t('devices.versionUnknown') }}</span></dd></div><div><dt>{{ t('devices.hardware') }}</dt><dd>{{ device.hostname ?? '—' }}</dd></div><div><dt>{{ t('devices.policy') }}</dt><dd>{{ policySummary(device.telemetry_policy) }}</dd></div><div><dt>{{ t('devices.lastSeen') }}</dt><dd>{{ device.last_seen_at ? new Date(device.last_seen_at).toLocaleString() : t('common.never') }}</dd></div></dl>
        <div v-if="rotatedCredential?.id===device.id" class="credential-reveal"><div><strong>{{ t('devices.credentialReady') }}</strong><button class="icon-button" :aria-label="t('common.close')" @click="rotatedCredential=null"><AppIcon name="close" :size="16" /></button></div><div class="copy-surface"><code>{{ rotatedCredential.credential }}</code><button class="copy-button" :title="t('devices.copyCredential')" :aria-label="t('devices.copyCredential')" @click="copyCredential"><AppIcon :name="copied ? 'check' : 'copy'" :size="17" /></button></div><small>{{ t('devices.credentialHint') }}</small></div>
        <footer><button class="button secondary" :disabled="!!device.revoked_at" @click="openPolicy(device)">{{ t('devices.configure') }}</button><button class="button secondary" :disabled="!!device.revoked_at" @click="rotate(device.id)">{{ t('devices.rotate') }}</button><button class="icon-button revoke-button" :disabled="!!device.revoked_at" @click="revoke(device.id)">{{ t('devices.revoke') }}</button></footer>
      </article>
      <div v-if="!devices.length" class="empty">{{ t('devices.noDevices') }}</div>
    </section>
  </div>
</template>

<style scoped>
.enrollment-panel,.policy-form{display:grid;gap:18px}.enrollment-fields{display:grid;grid-template-columns:1fr 1fr;gap:14px}.enrollment-fields :deep(.policy-picker){grid-column:1/-1}.enrollment-submit{justify-self:start}.policy-form .form-actions{display:flex;justify-content:flex-end;gap:9px}.command-reveal{display:grid;gap:10px;padding-top:4px}.command-reveal>p{margin:0;color:var(--muted);font-size:10px;line-height:1.55}.copy-surface{position:relative;min-width:0;display:flex;align-items:center;background:var(--input);border:1px solid var(--line);border-radius:9px}.copy-surface pre,.copy-surface code{min-width:0;display:block;flex:1;overflow:auto;margin:0;padding:14px 50px 14px 14px;color:var(--text);font-size:10px;white-space:pre}.copy-button{position:absolute;top:8px;right:8px;width:34px;height:34px;display:grid;place-items:center;color:var(--muted);background:var(--panel);border:1px solid var(--line-strong);border-radius:7px;cursor:pointer}.copy-button:hover{color:var(--accent);border-color:var(--accent)}.copy-feedback{justify-self:end;color:var(--success);font-size:9px}.device-roster{overflow:hidden}.roster-heading{min-height:60px;display:flex;align-items:center;justify-content:space-between;gap:12px;padding:13px 17px;border-bottom:1px solid var(--line)}.roster-heading h2{margin:0;font-size:16px;font-weight:600}.roster-heading span{color:var(--muted);font-size:10px}.device-row{display:grid;grid-template-columns:minmax(220px,.8fr) minmax(480px,1.5fr) auto;align-items:center;gap:18px;padding:18px 17px;border-bottom:1px solid var(--line)}.device-row:last-child{border-bottom:0}.device-row:hover{background:color-mix(in srgb,var(--panel-2) 38%,transparent)}.device-identity{display:grid;grid-template-columns:38px 1fr auto;align-items:center;gap:10px}.device-icon{width:37px;height:37px;display:grid;place-items:center;color:var(--accent);border:1px solid var(--line);border-radius:7px}.device-identity h2{margin:0;font-size:13px}.device-identity p{margin:3px 0 0;color:var(--muted);font-size:10px}.device-row dl{display:grid;grid-template-columns:repeat(4,1fr);margin:0}.device-row dl>div{min-width:0;padding:2px 11px;border-left:1px solid var(--line)}.device-row dt{color:var(--muted);font-size:9px}.device-row dd{margin:5px 0 0;overflow:hidden;font-size:10px;font-weight:500;text-overflow:ellipsis;white-space:nowrap}.device-row footer{display:flex;align-items:center;gap:7px}.device-row footer .button{min-height:35px;padding-inline:10px;font-size:9px}.revoke-button{color:var(--danger);font-size:9px}.credential-reveal{grid-column:1/-1;padding:12px;background:color-mix(in srgb,var(--warning) 7%,var(--panel));border:1px solid color-mix(in srgb,var(--warning) 30%,var(--line));border-radius:7px}.credential-reveal>div:first-child{display:flex;align-items:center;justify-content:space-between}.credential-reveal strong,.credential-reveal small,.credential-reveal code{display:block}.credential-reveal strong{color:var(--warning);font-size:9px}.credential-reveal .copy-surface{margin:8px 0}.credential-reveal code{padding-block:11px;font-size:9px}.credential-reveal small{color:var(--muted);font-size:9px}
.implementation-picker{grid-column:1/-1;max-width:420px}.implementation-picker small{color:var(--muted);font-size:9px;line-height:1.45}.agent-facts{color:var(--text)}.setup-steps{display:grid;gap:14px;margin:0;padding-left:20px;list-style:decimal}.setup-steps li::marker{color:var(--muted);font-size:9px;font-weight:600}.setup-steps li:only-child{margin-left:-20px;list-style:none}.setup-steps li>*+*{margin-top:8px}.step-text{margin:0;color:var(--muted);font-size:10px;line-height:1.55}.step-value{display:flex;align-items:center;gap:8px;margin:0}.step-value .inline-copy{margin-left:0}.step-value code{min-width:0;overflow-wrap:anywhere;color:var(--text);font-size:9px}.step-link{margin:0}.step-link a,.setup-docs a{color:var(--accent);font-size:10px}.setup-docs{margin:0}.reveal-context{color:var(--text)!important;font-weight:600}.enrollment-done{justify-self:end;margin-top:4px}.custom-connection{display:grid;gap:10px;padding-top:4px}.custom-connection>p{margin:0;color:var(--text);font-size:9px;line-height:1.5}.connection-facts{margin:0;border-top:1px solid var(--line)}.connection-facts>div{min-width:0;display:grid;grid-template-columns:minmax(150px,.35fr) minmax(0,1fr);align-items:center;gap:12px;padding:10px 0;border-bottom:1px solid var(--line)}.connection-facts dt{color:var(--muted);font-size:9px}.connection-facts dd{min-width:0;display:flex;align-items:center;gap:8px;margin:0}.connection-facts code{min-width:0;overflow-wrap:anywhere;color:var(--text);font-size:9px}.connection-facts small{color:var(--muted);font-size:8px}.inline-copy{flex:0 0 28px;width:28px;height:28px;display:grid;place-items:center;margin-left:auto;color:var(--muted);background:var(--panel);border:1px solid var(--line-strong);border-radius:6px;cursor:pointer}.inline-copy:hover,.inline-copy:focus-visible{color:var(--accent);border-color:var(--accent)}.custom-footer{display:flex;justify-content:space-between;gap:12px}.custom-footer a{color:var(--accent)}.custom-footer span{color:var(--text)}.version-value{display:flex;align-items:center;gap:6px}.version-pill{display:inline-flex;padding:3px 6px;border:1px solid;border-radius:999px;font-size:7px;font-weight:600;line-height:1;white-space:nowrap}.version-pill.warning{color:var(--warning);background:color-mix(in srgb,var(--warning) 9%,transparent);border-color:color-mix(in srgb,var(--warning) 38%,var(--line))}.version-pill.incompatible{color:var(--danger);background:color-mix(in srgb,var(--danger) 9%,transparent);border-color:color-mix(in srgb,var(--danger) 38%,var(--line))}
@media(max-width:1100px){.device-row{grid-template-columns:1fr auto}.device-row dl{grid-column:1/-1;grid-row:2}.device-row dl>div:first-child{border-left:0}.device-row footer{grid-column:2;grid-row:1}}
@media(max-width:700px){.enrollment-fields{grid-template-columns:1fr}.enrollment-fields :deep(.policy-picker){grid-column:auto}.implementation-picker{max-width:none}.connection-facts>div{grid-template-columns:1fr;gap:5px}.custom-footer{align-items:flex-start;flex-direction:column}.device-row{display:block}.device-row dl{grid-template-columns:1fr 1fr;margin:16px 0}.device-row dl>div{padding:9px;border-left:0;border-top:1px solid var(--line)}.device-row footer{flex-wrap:wrap;justify-content:flex-end}.device-identity{grid-template-columns:38px 1fr auto}}
</style>
