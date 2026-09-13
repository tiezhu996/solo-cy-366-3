<template>
  <div class="boot-code-page">
    <!-- 会员动态开机码 -->
    <template v-if="isMember">
      <van-notice-bar left-icon="info-o" text="到店后选择已确认预约生成开机码，5 分钟内有效，请向店员出示二维码" />

      <div v-if="activeCode" class="code-card">
        <div class="code-head">
          <span class="code-title">动态开机码</span>
          <StatusBadge kind="boot" :status="codeExpired ? 'expired' : activeCode.status" />
        </div>
        <QrCodeCard :text="encodeBootPayload(activeCode.code)" :display-text="activeCode.code" />
        <div class="countdown" :class="{ expired: codeExpired }">
          {{ codeExpired ? '开机码已失效，请重新生成' : `剩余有效时间 ${countdownText}` }}
        </div>
        <van-cell-group inset class="code-meta">
          <van-cell title="预约机位" :value="stationName(activeCode.station_id)" />
          <van-cell title="开机码状态" >
            <template #value><StatusBadge kind="boot" :status="activeCode.status" /></template>
          </van-cell>
          <van-cell title="失效时间" :value="formatTime(activeCode.expire_at)" />
        </van-cell-group>
        <div class="actions">
          <van-button type="primary" block round @click="regenerate">刷新开机码</van-button>
        </div>
      </div>

      <van-cell-group v-else inset title="选择已确认的预约生成开机码">
        <van-cell
          v-for="r in confirmedReservations"
          :key="r.id"
          :title="`${stationName(r.station_id)}（预约#${r.id}）`"
          :label="`${formatTime(r.start_time)} ~ ${formatTime(r.end_time)}`"
          is-link
          @click="onGenerate(r.id)"
        >
          <template #value>
            <van-button size="small" type="primary" :loading="generatingId === r.id">生成</van-button>
          </template>
        </van-cell>
        <EmptyState v-if="!confirmedReservations.length && loaded" description="暂无已确认的预约，请先在「预约」中预约机位" />
      </van-cell-group>
    </template>

    <!-- 非会员提示 -->
    <template v-else>
      <EmptyState description="仅会员可生成开机码，店员请前往「扫码开机」" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { showConfirmDialog, showSuccessToast, showToast } from 'vant'
import StatusBadge from '@/components/StatusBadge.vue'
import QrCodeCard from '@/components/QrCodeCard.vue'
import EmptyState from '@/components/EmptyState.vue'
import { useAuth } from '@/hooks/useAuth'
import { useStationStore } from '@/stores/stationStore'
import { listReservations, type Reservation } from '@/api/reservation'
import { generateBootCode, getMyBootCode, encodeBootPayload, type BootCode } from '@/api/bootCode'
import { BOOT_CODE_TTL_MINUTES } from '@/constants'
import { formatTime } from '@/utils/format'

const { isMember, user } = useAuth()
const stationStore = useStationStore()

const confirmedReservations = ref<Reservation[]>([])
const activeCode = ref<BootCode | null>(null)
const loaded = ref(false)
const generatingId = ref(0)
const nowTs = ref(Date.now())
let timer: ReturnType<typeof setInterval> | undefined

const codeExpired = computed(() => {
  if (!activeCode.value) return false
  return activeCode.value.status !== 'active' || nowTs.value >= new Date(activeCode.value.expire_at).getTime()
})

const countdownText = computed(() => {
  if (!activeCode.value) return ''
  const left = Math.max(0, new Date(activeCode.value.expire_at).getTime() - nowTs.value)
  const s = Math.floor(left / 1000)
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
})

function stationName(id: number) {
  const st = stationStore.stations.find((x) => x.id === id)
  return st ? `${st.name}（${st.area}）` : `机位#${id}`
}

async function loadReservations() {
  if (!user.value?.id) return
  const data = await listReservations({ page: 1, page_size: 50, status: 'confirmed', user_id: user.value.id })
  confirmedReservations.value = data.list
  loaded.value = true
}

async function loadMine() {
  const bc = await getMyBootCode()
  if (bc && bc.status === 'active' && new Date(bc.expire_at).getTime() > Date.now()) {
    activeCode.value = bc
  } else {
    activeCode.value = null
  }
}

async function onGenerate(reservationId: number) {
  generatingId.value = reservationId
  try {
    activeCode.value = await generateBootCode(reservationId)
    showSuccessToast('开机码已生成')
  } finally {
    generatingId.value = 0
  }
}

async function regenerate() {
  if (!activeCode.value) return
  try {
    await showConfirmDialog({ title: '刷新开机码', message: `刷新后原开机码立即作废，新码 ${BOOT_CODE_TTL_MINUTES} 分钟内有效，确定刷新吗？` })
  } catch {
    return
  }
  try {
    activeCode.value = await generateBootCode(activeCode.value.reservation_id)
    showSuccessToast('新开机码已生成')
  } catch {
    // 错误提示由请求拦截器统一处理
  }
}

onMounted(async () => {
  if (!isMember.value) {
    showToast('仅会员可生成开机码，店员请使用「扫码开机」')
    return
  }
  await stationStore.loadAll()
  await Promise.all([loadReservations(), loadMine()])
  timer = setInterval(() => { nowTs.value = Date.now() }, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.code-card { box-sizing: border-box; max-width: 100%; overflow: hidden; background: #fff; border-radius: 12px; margin: 12px 0; padding-bottom: 12px; }
.code-head { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px 0; }
.code-title { font-size: 16px; font-weight: 600; }
.code-meta { margin-top: 8px; }
.countdown { text-align: center; font-size: 15px; color: #07c160; font-weight: 600; margin: 6px 0 4px; }
.countdown.expired { color: #ee0a24; }
.actions { padding: 12px 16px 0; }
</style>
