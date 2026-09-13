<template>
  <div class="scan-boot">
    <van-notice-bar left-icon="scan" text="扫描会员动态开机码二维码，或手动输入 6 位开机码核销开机" />

    <template v-if="isStaffOrAdmin && !result">
      <van-cell-group inset class="scan-group">
        <div class="camera-wrap">
          <div id="boot-code-reader" class="reader"></div>
          <div v-if="cameraError" class="camera-mask">
            <van-icon name="photograph" size="36" />
            <div class="mask-text">摄像头不可用，请手动输入开机码</div>
          </div>
        </div>
        <van-field
          v-model="inputCode"
          center
          clearable
          label="开机码"
          placeholder="6 位数字开机码"
          type="digit"
          maxlength="6"
        >
          <template #button>
            <van-button size="small" type="primary" :loading="verifying" @click="submit(inputCode)">核销开机</van-button>
          </template>
        </van-field>
      </van-cell-group>

      <van-cell-group inset title="最近核销记录" class="record-group">
        <van-cell
          v-for="b in records"
          :key="b.id"
          :title="`开机码 ${b.code}`"
          :label="`核销时间：${b.verified_at ? formatTime(b.verified_at) : '-'}`"
        >
          <template #value><StatusBadge kind="boot" :status="b.status" /></template>
        </van-cell>
        <EmptyState v-if="!records.length" description="暂无核销记录" />
      </van-cell-group>
    </template>

    <BootResultPanel v-else-if="result" :detail="result" @again="reset" @view-sessions="goSessions" />
    <EmptyState v-else description="仅店员/管理员可扫码开机，请使用店员账号登录" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Html5Qrcode } from 'html5-qrcode'
import { showSuccessToast } from 'vant'
import StatusBadge from '@/components/StatusBadge.vue'
import EmptyState from '@/components/EmptyState.vue'
import BootResultPanel from '@/components/BootResultPanel.vue'
import { useAuth } from '@/hooks/useAuth'
import { listBootCodes, parseBootPayload, verifyBootCode, type BootCode, type BootSessionDetail } from '@/api/bootCode'
import { formatTime } from '@/utils/format'

const { isStaffOrAdmin } = useAuth()
const router = useRouter()

const inputCode = ref('')
const verifying = ref(false)
const result = ref<BootSessionDetail | null>(null)
const records = ref<BootCode[]>([])
const cameraError = ref(false)

let scanner: Html5Qrcode | undefined
let scanning = false
let lastCode = ''
let lastAt = 0

async function loadRecords() {
  try {
    const data = await listBootCodes({ page: 1, page_size: 10, status: 'used' })
    records.value = data.list
  } catch { /* 记录加载失败不阻塞扫码 */ }
}

async function submit(raw: string) {
  const code = parseBootPayload(raw)
  if (!/^\d{6}$/.test(code)) {
    return
  }
  if (verifying.value) return
  verifying.value = true
  try {
    result.value = await verifyBootCode({ code })
    showSuccessToast('开机成功')
    stopScanner()
    loadRecords()
  } catch {
    // 失败提示由请求拦截器统一展示；刷新记录以反映过期/已核销状态
    loadRecords()
  } finally {
    verifying.value = false
    inputCode.value = ''
  }
}

function onScan(raw: string) {
  const code = parseBootPayload(raw)
  if (!code) return
  // 防止摄像头短时间内重复识别同一码
  const now = Date.now()
  if (code === lastCode && now - lastAt < 3000) return
  lastCode = code
  lastAt = now
  submit(code)
}

async function startScanner() {
  try {
    scanner = new Html5Qrcode('boot-code-reader')
    await scanner.start(
      { facingMode: 'environment' },
      { fps: 10, qrbox: { width: 220, height: 220 } },
      onScan,
      () => { /* 单帧识别失败忽略 */ },
    )
    scanning = true
  } catch {
    cameraError.value = true
    scanning = false
  }
}

function stopScanner() {
  if (scanner && scanning) {
    scanner.stop().catch(() => { /* 停止异常忽略 */ })
    scanning = false
  }
}

function reset() {
  result.value = null
  inputCode.value = ''
  startScanner()
}

function goSessions() {
  router.push('/sessions')
}

onMounted(() => {
  if (!isStaffOrAdmin.value) return
  startScanner()
  loadRecords()
})

onUnmounted(stopScanner)
</script>

<style scoped>
.scan-group { margin-top: 12px; }
.camera-wrap { position: relative; width: 100%; aspect-ratio: 1; background: #000; overflow: hidden; border-radius: 8px 8px 0 0; }
.reader, .reader :deep(video), .reader :deep(img), .reader :deep(canvas) { width: 100% !important; height: 100% !important; border-radius: 8px 8px 0 0; }
.camera-mask { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; color: #c8c9cc; background: #f2f3f5; }
.mask-text { font-size: 13px; }
.record-group { margin-top: 12px; }
</style>
