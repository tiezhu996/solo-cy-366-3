<template>
  <div class="qr-box">
    <img v-if="dataUrl" :src="dataUrl" :alt="displayText || text" class="qr-img" :style="{ width: px + 'px', height: px + 'px' }" />
    <div v-else class="qr-empty">二维码生成失败</div>
    <div class="qr-code">{{ displayText || text }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import QRCode from 'qrcode'

// text：编码进二维码的完整载荷；displayText：码下方给人看的短文本（不传则展示 text）。
const props = withDefaults(defineProps<{ text: string; displayText?: string; size?: number }>(), {
  displayText: '',
  size: 200,
})
const px = computed(() => props.size)
const dataUrl = ref('')

async function render() {
  if (!props.text) {
    dataUrl.value = ''
    return
  }
  try {
    dataUrl.value = await QRCode.toDataURL(props.text, {
      width: props.size || 200,
      margin: 1,
      color: { dark: '#1d2129', light: '#ffffff' },
    })
  } catch {
    dataUrl.value = ''
  }
}

onMounted(render)
watch(() => props.text, render)
</script>

<style scoped>
.qr-box { box-sizing: border-box; width: 100%; max-width: 260px; margin: 0 auto; display: flex; flex-direction: column; align-items: center; padding: 12px; }
.qr-img { width: 200px; height: 200px; max-width: 100%; border-radius: 8px; }
.qr-empty { width: 200px; height: 200px; max-width: 100%; display: flex; align-items: center; justify-content: center; color: #969799; background: #f7f8fa; border-radius: 8px; }
.qr-code {
  box-sizing: border-box;
  width: 100%;
  margin-top: 10px;
  padding: 0 4px;
  text-align: center;
  font-size: clamp(20px, 7.5vw, 26px);
  font-weight: 700;
  letter-spacing: clamp(2px, 1.6vw, 6px);
  /* letter-spacing 会在末字后多出同样间距，用等量右内边距抵消以保持视觉居中 */
  padding-right: calc(4px + clamp(2px, 1.6vw, 6px));
  color: #1989fa;
  white-space: nowrap;
  overflow-wrap: anywhere;
}
</style>
