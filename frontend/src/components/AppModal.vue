<script lang="ts">
let openModalCount = 0
</script>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(defineProps<{ open: boolean; title: string; wide?: boolean; inactive?: boolean }>(), { wide: false, inactive: false })
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLElement>()
let previousFocus: HTMLElement | null = null
let ownsBodyLock = false

function lockBody(): void {
  if (ownsBodyLock) return
  ownsBodyLock = true
  openModalCount += 1
  document.body.classList.add('modal-open')
}

function unlockBody(): void {
  if (!ownsBodyLock) return
  ownsBodyLock = false
  openModalCount = Math.max(0, openModalCount - 1)
  if (!openModalCount) document.body.classList.remove('modal-open')
}

function close(): void { emit('close') }
function onKeydown(event: KeyboardEvent): void { if (props.open && event.key === 'Escape') close() }

watch(() => props.open, async (value) => {
  if (value) {
    previousFocus = document.activeElement as HTMLElement | null
    lockBody()
    await nextTick()
    const autofocus = dialog.value?.querySelector<HTMLElement>('[autofocus]')
    const firstControl = dialog.value?.querySelector<HTMLElement>('input, textarea, button, [tabindex="0"]')
    ;(autofocus ?? firstControl)?.focus()
  } else {
    unlockBody()
    previousFocus?.focus()
  }
}, { immediate: true })

document.addEventListener('keydown', onKeydown)
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  unlockBody()
})
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="app-modal-backdrop" @pointerdown.self="close">
      <section ref="dialog" :class="['app-modal panel', { wide, inactive }]" role="dialog" :aria-modal="!inactive" :aria-hidden="inactive || undefined" :inert="inactive || undefined" :aria-label="title">
        <div class="app-modal-heading">
          <div><slot name="eyebrow" /><h2>{{ title }}</h2></div>
          <button class="icon-button app-modal-close" type="button" :aria-label="$t('common.close')" @click="close"><AppIcon name="close" :size="18" /></button>
        </div>
        <div class="app-modal-content"><slot /></div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.app-modal-backdrop{position:fixed;inset:0;z-index:3000;display:grid;place-items:center;padding:20px;background:rgba(5,8,7,.64);backdrop-filter:blur(6px)}.app-modal{width:min(100%,520px);max-height:calc(100vh - 40px);display:flex;flex-direction:column;overflow:hidden;border-color:var(--line-strong);box-shadow:0 28px 90px rgba(0,0,0,.34)}.app-modal.wide{width:min(100%,940px)}.app-modal.inactive{filter:saturate(.7);pointer-events:none}.app-modal-heading{min-height:66px;display:flex;align-items:center;justify-content:space-between;gap:18px;padding:15px 18px;border-bottom:1px solid var(--line)}.app-modal-heading h2{margin:0;font-size:18px}.app-modal-heading :deep(.eyebrow){margin-bottom:4px}.app-modal-close{width:36px;height:36px;border:1px solid var(--line);border-radius:8px}.app-modal-content{min-height:0;overflow-y:auto;padding:20px}@media(max-width:620px){.app-modal-backdrop{align-items:end;padding:0}.app-modal,.app-modal.wide{width:100%;max-height:calc(100vh - 16px);border-radius:14px 14px 0 0}.app-modal-content{padding:17px}}
</style>

<style>
body.modal-open{overflow:hidden}
</style>
