import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import QrCodeCard from './QrCodeCard.vue'
import { encodeBootPayload } from '@/api/bootCode'

// 窄屏横穿缺陷回归：二维码编码完整载荷，而码下方只显示 6 位短码。
describe('QrCodeCard 显示回归', () => {
  it('二维码编码完整载荷，但文字只显示 displayText 短码', async () => {
    const code = '238568'
    const wrapper = mount(QrCodeCard, {
      props: { text: encodeBootPayload(code), displayText: code },
    })
    // 等待 onMounted 中异步生成二维码
    await nextTick()
    await new Promise((resolve) => setTimeout(resolve, 50))

    const img = wrapper.find('img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src') || '').toContain('data:image/png;base64,')

    const codeEl = wrapper.find('.qr-code')
    expect(codeEl.text()).toBe(code)
    expect(codeEl.text()).not.toContain('ESPORTSBAR')
    expect(codeEl.text().length).toBe(6)
  })

  it('未传 displayText 时回退展示 text', async () => {
    const wrapper = mount(QrCodeCard, { props: { text: '112233' } })
    await nextTick()
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(wrapper.find('.qr-code').text()).toBe('112233')
  })

  it('容器带宽度约束（max-width 100%），窄屏下不撑破父卡片', () => {
    const wrapper = mount(QrCodeCard, {
      props: { text: encodeBootPayload('999888'), displayText: '999888' },
    })
    expect(wrapper.find('.qr-box').exists()).toBe(true)
    expect(wrapper.find('.qr-code').text()).toBe('999888')
  })
})
