import { get, post } from '@/utils/request'

export interface BootCode {
  id: number
  code: string
  reservation_id: number
  user_id: number
  station_id: number
  session_id: number
  status: string
  expire_at: string
  verified_by: number
  verified_at: string | null
  created_at: string
}

// 扫码核销成功后返回的上机详情。
export interface BootSessionDetail {
  session_id: number
  code: string
  reservation_id: number
  user_id: number
  username: string
  nickname: string
  phone: string
  station_id: number
  station_name: string
  area: string
  station_type: string
  start_time: string
  reserved_end_time: string
  price_per_hour: number
  package_hours: number
  balance_cost: number
  remaining_hours: number
  balance: number
  status: string
  expire_at: string
  verified_at: string | null
}

// 会员基于已确认预约生成动态开机码。
export function generateBootCode(reservationId: number) {
  return post<BootCode>('/boot-codes', { reservation_id: reservationId })
}

// 会员查询当前有效开机码。
export function getMyBootCode(reservationId?: number) {
  return get<BootCode | null>('/boot-codes/mine', reservationId ? { reservation_id: reservationId } : undefined)
}

// 店员扫码校验并开机。
export function verifyBootCode(data: { code: string; station_id?: number }) {
  return post<BootSessionDetail>('/boot-codes/verify', data)
}

// 店员/管理员查询核销记录。
export function listBootCodes(params: { page: number; page_size: number; status?: string }) {
  return get<{ list: BootCode[]; total: number }>('/boot-codes', params)
}

// 二维码载荷前缀，扫码后用于识别本系统开机码。
export const BOOT_CODE_QR_PREFIX = 'ESPORTSBAR:BOOT:'

// 将动态码编码为二维码内容。
export function encodeBootPayload(code: string) {
  return `${BOOT_CODE_QR_PREFIX}${code}`
}

// 从扫码内容解析动态码，非本系统码返回空串。
export function parseBootPayload(raw: string) {
  const value = (raw || '').trim()
  if (value.startsWith(BOOT_CODE_QR_PREFIX)) return value.slice(BOOT_CODE_QR_PREFIX.length)
  return /^\d{6}$/.test(value) ? value : ''
}
