const args = process.argv.slice(2)

function readFlag(name, fallback = '') {
  const index = args.indexOf(name)
  if (index === -1) return fallback
  return args[index + 1] || fallback
}

const apiBase = (readFlag('--api', process.env.VOLA_API_BASE || 'http://127.0.0.1:42720/api')).replace(/\/+$/, '')
const uiRoute = buildUiRoute(apiBase)
const suffix = normalizeSlug(readFlag('--suffix', Date.now().toString(36).slice(-8)))
if (!suffix) {
  console.error('Invalid suffix. Use lowercase letters, numbers, or hyphens.')
  process.exit(1)
}

const memberSlug = `demo-member-${suffix}`
const teamSlug = `local-demo-${suffix}`
const skillSlug = `local-demo-review-${suffix}`
const agentSlug = `local-demo-agent-${suffix}`
const password = `VolaDemo-${suffix}-123!`

function normalizeSlug(value) {
  return String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 32)
}

function buildUiRoute(base) {
  try {
    const url = new URL(base)
    url.pathname = '/team'
    url.search = ''
    url.hash = ''
    return url.toString()
  } catch {
    return 'http://127.0.0.1:42720/team'
  }
}

async function api(path, options = {}) {
  const headers = { ...(options.headers || {}) }
  if (options.token) headers.Authorization = `Bearer ${options.token}`
  let body = options.body
  if (body !== undefined && typeof body !== 'string') {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(body)
  }
  const response = await fetch(`${apiBase}${path}`, {
    method: options.method || 'GET',
    headers,
    body,
  })
  const text = await response.text()
  let parsed = null
  try {
    parsed = text ? JSON.parse(text) : null
  } catch {
    parsed = { raw: text }
  }
  if (!response.ok) {
    const message = parsed?.message || parsed?.error || text || response.statusText
    const error = new Error(`${options.method || 'GET'} ${path} -> ${response.status}: ${message}`)
    error.status = response.status
    error.data = parsed
    throw error
  }
  return parsed && Object.hasOwn(parsed, 'ok') ? parsed.data : parsed
}

function record(steps, name, extra = {}) {
  steps.push({ name, ...extra })
}

function isReusableConflict(error) {
  if (error?.status !== 400 && error?.status !== 409) return false
  const message = String(error?.message || '').toLowerCase()
  return message.includes('already') ||
    message.includes('duplicate') ||
    message.includes('unique') ||
    message.includes('taken') ||
    message.includes('exists')
}

function teamsFromResponse(value) {
  if (Array.isArray(value)) return value
  if (Array.isArray(value?.teams)) return value.teams
  return []
}

async function findTeamBySlug(token, slug) {
  const listed = await api('/teams', { token })
  return teamsFromResponse(listed).find((team) => team.slug === slug) || null
}

async function main() {
  const steps = []
  const config = await api('/config')
  if (!config?.local_mode) {
    throw new Error(`Refusing to seed a non-local Vola server at ${apiBase}`)
  }
  record(steps, 'config-local-mode')

  const ownerTokenResponse = await api('/local/owner-token', { method: 'POST' })
  const ownerToken = ownerTokenResponse?.token
  if (!ownerToken) throw new Error('Local owner token response did not include a token')
  record(steps, 'owner-token')

  const memberEmail = `${memberSlug}@example.local`
  try {
    const createdUser = await api('/admin/users', {
      method: 'POST',
      token: ownerToken,
      body: {
        email: memberEmail,
        password,
        display_name: `Demo Member ${suffix}`,
        slug: memberSlug,
      },
    })
    record(steps, 'create-member', { member_slug: createdUser.user?.slug || memberSlug })
  } catch (err) {
    if (!isReusableConflict(err)) throw err
    record(steps, 'reuse-member', { member_slug: memberSlug })
  }

  const login = await api('/auth/login', {
    method: 'POST',
    body: { email: memberEmail, password },
  })
  const memberToken = login?.access_token
  if (!memberToken) throw new Error('Member login did not include an access token')
  record(steps, 'login-member')

  let team = null
  try {
    const createdTeam = await api('/teams', {
      method: 'POST',
      token: ownerToken,
      body: {
        slug: teamSlug,
        name: `Local Demo Team ${suffix}`,
        description: 'Local-only team simulation for UI verification.',
      },
    })
    team = createdTeam.team
    record(steps, 'create-team', { team_id: team?.id, team_slug: team?.slug })
  } catch (err) {
    if (!isReusableConflict(err)) throw err
    team = await findTeamBySlug(ownerToken, teamSlug)
    if (!team?.id) {
      throw new Error(`Team slug ${teamSlug} already exists but is not visible to the local owner`)
    }
    record(steps, 'reuse-team', { team_id: team.id, team_slug: team.slug })
  }
  if (!team?.id) throw new Error('Team creation response did not include a team id')

  const member = await api(`/teams/${encodeURIComponent(team.id)}/members`, {
    method: 'POST',
    token: ownerToken,
    body: { user_slug: memberSlug, role: 'member' },
  })
  record(steps, 'add-member', { role: member.member?.role || 'member' })

  const skillPath = `/skills/${skillSlug}`
  await api(`/teams/${encodeURIComponent(team.id)}/tree${skillPath}/SKILL.md`, {
    method: 'PUT',
    token: memberToken,
    body: {
      content: `# Local Demo Skill ${suffix}\n\nUsed to verify local team review, publishing, install, and update notices.\n`,
      mime_type: 'text/markdown',
      min_trust_level: 2,
      metadata: { purpose: 'local-team-simulation-ui-test', demo_suffix: suffix },
    },
  })
  record(steps, 'write-team-skill', { skill_path: skillPath })

  await api(`/teams/${encodeURIComponent(team.id)}/skill-review-requests`, {
    method: 'POST',
    token: memberToken,
    body: { skill_path: skillPath, note: 'Demo member requests review.' },
  })
  record(steps, 'request-skill-review')

  await api(`/teams/${encodeURIComponent(team.id)}/skill-review-requests/resolve`, {
    method: 'POST',
    token: ownerToken,
    body: { skill_path: skillPath, decision: 'approved', note: 'Approved for local demo.' },
  })
  record(steps, 'approve-skill-review')

  const copied = await api('/skills/copy-to-personal', {
    method: 'POST',
    token: memberToken,
    body: { team_id: team.id, source_path: skillPath, overwrite: true },
  })
  record(steps, 'member-install-skill', { files: copied.files, target_path: copied.target_path, overwrite: copied.overwrite })

  await api(`/teams/${encodeURIComponent(team.id)}/tree${skillPath}/SKILL.md`, {
    method: 'PUT',
    token: ownerToken,
    body: {
      content: `# Local Demo Skill ${suffix}\n\nVersion 2 for local team update notice verification.\n`,
      mime_type: 'text/markdown',
      min_trust_level: 2,
      metadata: { purpose: 'local-team-simulation-ui-test', demo_suffix: suffix, version: 2 },
    },
  })
  record(steps, 'update-team-skill')

  const checked = await api(`/teams/${encodeURIComponent(team.id)}/skill-subscriptions/check`, {
    method: 'POST',
    token: ownerToken,
  })
  record(steps, 'check-subscriptions', { notifications: checked.notifications?.length || 0 })

  await api(`/teams/${encodeURIComponent(team.id)}/agents`, {
    method: 'POST',
    token: ownerToken,
    body: {
      slug: agentSlug,
      name: `Local Demo Agent ${suffix}`,
      description: 'Team agent recipe card for local simulation UI verification.',
      instructions: 'Read the team skill and propose actions within approval boundaries.',
      default_skill_paths: [skillPath],
      target_agents: ['codex', 'claude-code', 'cursor', 'gemini-cli'],
      model: 'local-demo',
      permissions: ['read-team-skill'],
      approval_required: ['write-production-data'],
      status: 'published',
      visibility: 'team',
    },
  })
  record(steps, 'publish-agent-recipe', { agent_slug: agentSlug })

  await api(`/teams/${encodeURIComponent(team.id)}/agents/${encodeURIComponent(agentSlug)}/install`, {
    method: 'POST',
    token: memberToken,
  })
  record(steps, 'member-install-agent')

  const report = await api(`/teams/${encodeURIComponent(team.id)}/skill-subscription-report`, {
    token: ownerToken,
  })
  const history = await api(`/teams/${encodeURIComponent(team.id)}/skill-review-history`, {
    token: ownerToken,
  })
  const notices = await api(`/teams/${encodeURIComponent(team.id)}/skill-update-notifications`, {
    token: ownerToken,
  })
  const targetSkill = (report.skills || []).find((item) => item.skill_path === skillPath)
  const memberStatus = targetSkill?.members?.find((item) => item.user_slug === memberSlug)?.status || null
  if (memberStatus !== 'update_available') {
    throw new Error(`Expected member subscription status update_available, got ${memberStatus || 'missing'}`)
  }

  console.log(JSON.stringify({
    ok: true,
    api_base: apiBase,
    ui_route: uiRoute,
    suffix,
    team: { id: team.id, slug: team.slug, name: team.name },
    member: { slug: memberSlug, email: memberEmail },
    skill: { path: skillPath, member_subscription_status: memberStatus },
    agent: { slug: agentSlug },
    reused_existing: steps.some((step) => step.name.startsWith('reuse-')),
    review_events: history.events?.length || 0,
    update_notifications: notices.notifications?.length || 0,
    latest_notification_status: notices.notifications?.[0]?.status || null,
    steps,
  }, null, 2))
}

main().catch((err) => {
  console.error(err?.message || String(err))
  process.exit(1)
})
