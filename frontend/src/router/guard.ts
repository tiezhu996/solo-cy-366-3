// 路由守卫判定纯函数：便于单元测试覆盖"未登录 / 角色不符 / 放行"三类分支。
export interface GuardInput {
  token: string
  role?: string
  name: unknown
  roles?: string[]
}

export type GuardResult = true | { name: 'Login' } | { name: 'Dashboard' }

export function resolveRouteGuard(input: GuardInput): GuardResult {
  if (input.name !== 'Login' && !input.token) {
    return { name: 'Login' }
  }
  if (input.name === 'Login' && input.token) {
    return { name: 'Dashboard' }
  }
  if (input.roles && input.role !== undefined && !input.roles.includes(input.role)) {
    return { name: 'Dashboard' }
  }
  return true
}
