import { describe, expect, it } from 'vitest'
import { resolveRouteGuard } from './guard'
import { USER_ROLE } from '@/constants'

// 路由角色矩阵：对应后端 router/boot_code.go 的 RBAC 组合。
describe('resolveRouteGuard 扫码页面角色守卫', () => {
  it('未登录访问任意业务页 -> 跳登录', () => {
    expect(resolveRouteGuard({ token: '', name: 'BootCode' })).toEqual({ name: 'Login' })
  })

  it('已登录访问登录页 -> 跳看板', () => {
    expect(resolveRouteGuard({ token: 'jwt', role: USER_ROLE.MEMBER, name: 'Login' })).toEqual({ name: 'Dashboard' })
  })

  it('会员可进入「我的开机码」，店员/管理员不行', () => {
    const roles = [USER_ROLE.MEMBER]
    expect(resolveRouteGuard({ token: 'jwt', role: USER_ROLE.MEMBER, name: 'BootCode', roles })).toBe(true)
    expect(resolveRouteGuard({ token: 'jwt', role: USER_ROLE.STAFF, name: 'BootCode', roles })).toEqual({ name: 'Dashboard' })
    expect(resolveRouteGuard({ token: 'jwt', role: USER_ROLE.ADMIN, name: 'BootCode', roles })).toEqual({ name: 'Dashboard' })
  })

  it('店员/管理员可进入「扫码开机」，会员不行', () => {
    const roles = [USER_ROLE.ADMIN, USER_ROLE.STAFF]
    expect(resolveRouteGuard({ token: 'jwt', role: USER_ROLE.STAFF, name: 'ScanBoot', roles })).toBe(true)
    expect(resolveRouteGuard({ token: 'jwt', role: USER_ROLE.ADMIN, name: 'ScanBoot', roles })).toBe(true)
    expect(resolveRouteGuard({ token: 'jwt', role: USER_ROLE.MEMBER, name: 'ScanBoot', roles })).toEqual({ name: 'Dashboard' })
  })

  it('未声明 roles 的页面登录后一律放行', () => {
    expect(resolveRouteGuard({ token: 'jwt', role: USER_ROLE.MEMBER, name: 'Dashboard' })).toBe(true)
  })
})
