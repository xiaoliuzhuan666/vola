import { test, expect, type Page } from '@playwright/test'
import { setupUser } from './helpers'

async function openProfileEditor(page: Page) {
  await page.getByRole('button', { name: /编辑资料|Edit profile/ }).click()
  await expect(page.getByRole('dialog', { name: /编辑个人资料|Edit profile/ })).toBeVisible()
}

function profileField(page: Page, name: RegExp) {
  return page.getByRole('dialog', { name: /编辑个人资料|Edit profile/ }).getByLabel(name)
}

test.describe('Info Page - Profile Persistence', () => {
  test('save preferences and verify after reload', async ({ page, request }) => {
    await setupUser(page, request)
    await page.goto('/data/profile')
    await page.waitForLoadState('networkidle')

    await openProfileEditor(page)
    await profileField(page, /工作偏好|Work preferences/).fill('偏好简洁代码，Go 优先')
    await page.getByRole('button', { name: /保存资料|Save profile/ }).click()
    await expect(page.getByText('已保存')).toBeVisible({ timeout: 5000 })

    await page.reload()
    await page.waitForLoadState('networkidle')
    await openProfileEditor(page)
    expect(await profileField(page, /工作偏好|Work preferences/).inputValue()).toContain('偏好简洁代码')
  })

  test('save relationships and verify after reload', async ({ page, request }) => {
    await setupUser(page, request)
    await page.goto('/data/profile')
    await page.waitForLoadState('networkidle')

    await openProfileEditor(page)
    await profileField(page, /人际关系|Relationships/).fill('Alice 是产品经理')
    await page.getByRole('button', { name: /保存资料|Save profile/ }).click()
    await expect(page.getByText('已保存')).toBeVisible({ timeout: 5000 })

    await page.reload()
    await page.waitForLoadState('networkidle')
    await openProfileEditor(page)
    expect(await profileField(page, /人际关系|Relationships/).inputValue()).toContain('Alice')
  })

  test('save principles and verify after reload', async ({ page, request }) => {
    await setupUser(page, request)
    await page.goto('/data/profile')
    await page.waitForLoadState('networkidle')

    await openProfileEditor(page)
    await profileField(page, /决策风格|Decision style/).fill('先做再说，最小可行')
    await page.getByRole('button', { name: /保存资料|Save profile/ }).click()
    await expect(page.getByText('已保存')).toBeVisible({ timeout: 5000 })

    await page.reload()
    await page.waitForLoadState('networkidle')
    await openProfileEditor(page)
    expect(await profileField(page, /决策风格|Decision style/).inputValue()).toContain('先做再说')
  })

  test('save all three with single button', async ({ page, request }) => {
    await setupUser(page, request)
    await page.goto('/data/profile')
    await page.waitForLoadState('networkidle')

    await openProfileEditor(page)
    await profileField(page, /工作偏好|Work preferences/).fill('偏好 TypeScript')
    await profileField(page, /人际关系|Relationships/).fill('Bob 是设计师')
    await profileField(page, /决策风格|Decision style/).fill('代码即文档')

    await page.getByRole('button', { name: /保存资料|Save profile/ }).click()
    await expect(page.getByText('已保存')).toBeVisible({ timeout: 5000 })

    await page.reload()
    await page.waitForLoadState('networkidle')

    await openProfileEditor(page)
    expect(await profileField(page, /工作偏好|Work preferences/).inputValue()).toContain('TypeScript')
    expect(await profileField(page, /人际关系|Relationships/).inputValue()).toContain('Bob')
    expect(await profileField(page, /决策风格|Decision style/).inputValue()).toContain('代码即文档')
  })

  test('privacy actions stay available on page', async ({ page, request }) => {
    await setupUser(page, request)
    await page.goto('/data/profile')
    await page.waitForLoadState('networkidle')

    await expect(page.getByRole('heading', { name: /隐私操作|Privacy Actions/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /导出全部数据|Export all data/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /撤销全部 token|Revoke all tokens/ })).toBeVisible()
  })
})
