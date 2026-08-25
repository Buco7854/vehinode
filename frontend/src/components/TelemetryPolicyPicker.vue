<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TelemetryPolicy } from '../api/types'
import AppSelect from './AppSelect.vue'

type PolicyPreset = 'dataSaver' | 'balanced' | 'lightLive' | 'live' | 'custom'

const props = defineProps<{ modelValue: TelemetryPolicy }>()
const emit = defineEmits<{ 'update:modelValue': [value: TelemetryPolicy] }>()
const { t } = useI18n()
const inputId = useId()
const customSelected = ref(false)

const presets: Array<{ id: Exclude<PolicyPreset, 'custom'>; driving: number; parked: number }> = [
  { id: 'dataSaver', driving: 120, parked: 900 },
  { id: 'balanced', driving: 30, parked: 900 },
  { id: 'lightLive', driving: 5, parked: 900 },
  { id: 'live', driving: 1, parked: 900 },
]

const selectedPreset = computed<PolicyPreset>(() => {
  if (customSelected.value) return 'custom'
  return presets.find(({ driving, parked }) => (
    props.modelValue.sampling_seconds === driving
    && props.modelValue.upload_seconds === driving
    && props.modelValue.parked_sampling_seconds === parked
    && props.modelValue.parked_upload_seconds === parked
  ))?.id ?? 'custom'
})
const invalid = computed(() => (
  props.modelValue.sampling_seconds < 1
  || props.modelValue.sampling_seconds > 86400
  || props.modelValue.parked_sampling_seconds < 1
  || props.modelValue.parked_sampling_seconds > 86400
))

function choosePreset(value: string | number | null): void {
  const id = value as PolicyPreset
  if (id === 'custom') {
    customSelected.value = true
    return
  }
  const preset = presets.find((item) => item.id === id)
  if (!preset) return
  customSelected.value = false
  emit('update:modelValue', {
    sampling_seconds: preset.driving,
    upload_seconds: preset.driving,
    parked_sampling_seconds: preset.parked,
    parked_upload_seconds: preset.parked,
  })
}

function setInterval(state: 'driving' | 'parked', event: Event): void {
  const value = Number((event.target as HTMLInputElement).value)
  customSelected.value = true
  if (state === 'driving') {
    emit('update:modelValue', {
      ...props.modelValue,
      sampling_seconds: value,
      upload_seconds: value,
    })
    return
  }
  emit('update:modelValue', {
    ...props.modelValue,
    parked_sampling_seconds: value,
    parked_upload_seconds: value,
  })
}
</script>

<template>
  <fieldset class="policy-picker" :aria-describedby="`${inputId}-hint`">
    <legend>{{ t('devices.policyTitle') }}</legend>
    <p :id="`${inputId}-hint`" class="policy-intro">{{ t('devices.policyHint') }}</p>
    <label class="field preset-field">
      <span>{{ t('devices.policyPreset') }}</span>
      <AppSelect :model-value="selectedPreset" :aria-label="t('devices.policyPreset')" @update:model-value="choosePreset">
        <option v-for="preset in presets" :key="preset.id" :value="preset.id">{{ t(`devices.presets.${preset.id}.name`) }}</option>
        <option value="custom">{{ t('devices.presets.custom.name') }}</option>
      </AppSelect>
    </label>
    <div class="interval-fields">
      <label class="field">
        <span>{{ t('devices.drivingIntervalSeconds') }}</span>
        <input class="input" type="number" min="1" max="86400" required :value="modelValue.sampling_seconds" @input="setInterval('driving', $event)">
        <small>{{ t('devices.drivingIntervalHint') }}</small>
      </label>
      <label class="field">
        <span>{{ t('devices.parkedIntervalSeconds') }}</span>
        <input class="input" type="number" min="1" max="86400" required :value="modelValue.parked_sampling_seconds" @input="setInterval('parked', $event)">
        <small>{{ t('devices.parkedIntervalHint') }}</small>
      </label>
    </div>
    <p v-if="selectedPreset !== 'custom'" class="estimate-note"><strong>{{ t(`devices.presets.${selectedPreset}.estimate`) }}</strong> · {{ t('devices.estimateBasis') }}</p>
    <p v-if="invalid" class="error" role="alert">{{ t('devices.policyInvalid') }}</p>
  </fieldset>
</template>

<style scoped>
.policy-picker{min-width:0;margin:0;padding:0;border:0}.policy-picker legend{padding:0;color:var(--text);font-size:12px;font-weight:600}.policy-intro{margin:6px 0 13px;color:var(--muted);font-size:10px;line-height:1.55}.preset-field{max-width:320px}.interval-fields{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-top:12px}.interval-fields small{color:var(--muted);font-size:9px;line-height:1.4}.estimate-note{margin:11px 0 0;color:var(--muted);font-size:9px;line-height:1.45}.estimate-note strong{color:var(--text)}.policy-picker>.error{margin:11px 0 0}@media(max-width:640px){.preset-field{max-width:none}.interval-fields{grid-template-columns:1fr}}
</style>
