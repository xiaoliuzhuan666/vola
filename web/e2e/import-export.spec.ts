import { test, expect } from '@playwright/test'
import { registerUser, loginViaUI, setupUser } from './helpers'

test.describe('Import & Export', () => {
  test('export JSON from dashboard', async ({ page, request }) => {
    await setupUser(page, request)
    await page.goto('/settings/profile')
    await page.waitForLoadState('networkidle')

    // Click JSON export
    const [download] = await Promise.all([
      page.waitForEvent('download', { timeout: 10000 }).catch(() => null),
      page.getByRole('button', { name: /导出全部数据|Export all data/ }).click(),
    ])

    // Either a download started or success message shown
    const hasMsg = await page.getByText('已开始下载').isVisible({ timeout: 3000 }).catch(() => false)
    expect(download !== null || hasMsg).toBeTruthy()
  })

  test('import skill then verify in skills library', async ({ page, request }) => {
    const user = await registerUser(request)

    // Import skill via API
    await request.post('/api/import/skill', {
      headers: { Authorization: `Bearer ${user.token}` },
      data: {
        name: 'pw-test-skill',
        files: { 'SKILL.md': '# Test Skill\nPlaywright imported skill' },
      },
    })

    // Login and check skills library
    await loginViaUI(page, user.email, user.password)
    await page.goto('/skills')
    await page.waitForLoadState('networkidle')

    await expect(page.getByText('pw-test-skill').first()).toBeVisible({ timeout: 10000 })
  })

  test('import profile then verify on info page', async ({ page, request }) => {
    const user = await registerUser(request)

    // Import profile via API
    await request.post('/api/import/profile', {
      headers: { Authorization: `Bearer ${user.token}` },
      data: {
        preferences: 'Imported preference via Playwright test',
        relationships: 'Carol is a colleague',
        principles: 'Imported principle via Playwright test',
      },
    })

    // Login and check info page
    await loginViaUI(page, user.email, user.password)
    await page.goto('/info')
    await page.waitForLoadState('networkidle')

    await page.getByRole('button', { name: /编辑资料|Edit profile/ }).click()
    await expect(page.getByRole('dialog', { name: /编辑个人资料|Edit profile/ })).toBeVisible()

    const dialog = page.getByRole('dialog', { name: /编辑个人资料|Edit profile/ })

    const prefValue = await dialog.getByLabel(/工作偏好|Work preferences/).inputValue()
    expect(prefValue).toContain('Imported preference')

    const relValue = await dialog.getByLabel(/人际关系|Relationships/).inputValue()
    expect(relValue).toContain('Carol')

    const principleValue = await dialog.getByLabel(/决策风格|Decision style/).inputValue()
    expect(principleValue).toContain('Imported principle')
  })
})
