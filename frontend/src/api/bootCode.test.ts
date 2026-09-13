import { describe, expect, it } from 'vitest'
import { BOOT_CODE_QR_PREFIX, encodeBootPayload, parseBootPayload } from './bootCode'

// 扫码解析闭环：店员摄像头扫到的字符串可能是带前缀的二维码内容，也可能是纯 6 位码或脏数据。
describe('parseBootPayload 开机码解析', () => {
  it('解析带业务前缀的二维码内容', () => {
    expect(parseBootPayload(`${BOOT_CODE_QR_PREFIX}123456`)).toBe('123456')
  })

  it('兼容直接输入的 6 位数字码', () => {
    expect(parseBootPayload('654321')).toBe('654321')
  })

  it('容忍首尾空白', () => {
    expect(parseBootPayload(`  ${BOOT_CODE_QR_PREFIX}000999\n`)).toBe('000999')
  })

  it('拒绝非本系统二维码、非法长度与非数字内容', () => {
    expect(parseBootPayload('https://example.com/q/123456')).toBe('')
    expect(parseBootPayload('12345')).toBe('')
    expect(parseBootPayload('1234567')).toBe('')
    expect(parseBootPayload('abcdef')).toBe('')
    expect(parseBootPayload('')).toBe('')
  })

  it('encodeBootPayload 与 parseBootPayload 互为逆运算（前端展示码与二维码载荷分离）', () => {
    const code = '238568'
    expect(encodeBootPayload(code)).toBe(`${BOOT_CODE_QR_PREFIX}${code}`)
    expect(parseBootPayload(encodeBootPayload(code))).toBe(code)
  })
})
