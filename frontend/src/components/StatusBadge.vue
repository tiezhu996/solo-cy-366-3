<template>
  <van-tag :type="tagType" plain>{{ text }}</van-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  STATION_STATUS_TEXT, STATION_STATUS_TYPE,
  RESERVATION_STATUS_TEXT, RESERVATION_STATUS_TYPE,
  TOURNAMENT_STATUS_TEXT, TOURNAMENT_STATUS_TYPE,
  SESSION_STATUS_TEXT,
  BOOT_CODE_STATUS_TEXT, BOOT_CODE_STATUS_TYPE,
} from '@/constants'

const props = defineProps<{ kind: 'station' | 'reservation' | 'tournament' | 'session' | 'boot'; status: string }>()

const text = computed(() => {
  switch (props.kind) {
    case 'station': return STATION_STATUS_TEXT[props.status] || props.status
    case 'reservation': return RESERVATION_STATUS_TEXT[props.status] || props.status
    case 'tournament': return TOURNAMENT_STATUS_TEXT[props.status] || props.status
    case 'session': return SESSION_STATUS_TEXT[props.status] || props.status
    case 'boot': return BOOT_CODE_STATUS_TEXT[props.status] || props.status
  }
})

type VanTagType = 'primary' | 'success' | 'warning' | 'danger' | 'default'

const tagType = computed<VanTagType>(() => {
  switch (props.kind) {
    case 'station': return (STATION_STATUS_TYPE[props.status] as VanTagType) || 'default'
    case 'reservation': return (RESERVATION_STATUS_TYPE[props.status] as VanTagType) || 'default'
    case 'tournament': return (TOURNAMENT_STATUS_TYPE[props.status] as VanTagType) || 'default'
    case 'boot': return (BOOT_CODE_STATUS_TYPE[props.status] as VanTagType) || 'default'
    default: return 'default'
  }
})
</script>
