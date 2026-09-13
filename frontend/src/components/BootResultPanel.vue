<template>
  <div class="boot-result">
    <div class="result-head">
      <van-icon name="passed" color="#07c160" size="48" />
      <div class="result-title">开机成功</div>
      <div class="result-sub">扫码校验通过，机位已占用并开始计费</div>
    </div>
    <van-cell-group inset title="上机详情">
      <van-cell title="会员账号" :value="`${detail.nickname || detail.username}（${detail.username}）`" />
      <van-cell v-if="detail.phone" title="手机号" :value="detail.phone" />
      <van-cell title="机位" :value="`${detail.station_name} · ${detail.area}`" />
      <van-cell title="开始时间" :value="formatTime(detail.start_time)" />
      <van-cell title="预约结束时间" :value="formatTime(detail.reserved_end_time)" />
      <van-cell title="机位时价" :value="`¥${detail.price_per_hour.toFixed(2)}/小时`" />
    </van-cell-group>
    <van-cell-group inset title="本次扣费">
      <van-cell title="时长包扣减" :value="`${detail.package_hours.toFixed(2)} 小时`" :value-class="detail.package_hours > 0 ? 'pay-pkg' : ''" />
      <van-cell title="余额扣减" :value="formatMoney(detail.balance_cost)" :value-class="detail.balance_cost > 0 ? 'pay-bal' : ''" />
      <van-cell title="剩余时长包" :value="`${detail.remaining_hours.toFixed(2)} 小时`" />
      <van-cell title="剩余余额" :value="formatMoney(detail.balance)" />
    </van-cell-group>
    <div class="result-actions">
      <van-button type="primary" block round @click="emit('again')">继续扫码开机</van-button>
      <van-button plain block round class="second" @click="emit('viewSessions')">查看上机记录</van-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { BootSessionDetail } from '@/api/bootCode'
import { formatTime, formatMoney } from '@/utils/format'

defineProps<{ detail: BootSessionDetail }>()
const emit = defineEmits<{
  (e: 'again'): void
  (e: 'viewSessions'): void
}>()
</script>

<style scoped>
.boot-result { padding: 4px 0 12px; }
.result-head { text-align: center; padding: 20px 0 14px; }
.result-title { font-size: 20px; font-weight: 700; margin-top: 8px; color: #1d2129; }
.result-sub { font-size: 13px; color: #969799; margin-top: 4px; }
.result-actions { padding: 16px; display: flex; flex-direction: column; gap: 10px; }
.second { margin: 0; }
:deep(.pay-pkg) { color: #07c160; font-weight: 600; }
:deep(.pay-bal) { color: #ee0a24; font-weight: 600; }
</style>
