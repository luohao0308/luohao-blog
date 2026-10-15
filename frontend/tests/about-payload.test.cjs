const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const { test } = require('node:test')
const vm = require('node:vm')
const ts = require('typescript')

test('About updates submit every writable field without GET response metadata', async () => {
  const requests = []
  const source = readFileSync(path.resolve(__dirname, '../app/composables/usePortfolio.ts'), 'utf8')
    .replace(/^import .*$/gm, '')
    .replaceAll('import.meta.client', 'false')
  const context = {
    exports: {},
    useAuth: () => ({
      authFetch: async (url, options) => {
        requests.push({ url, options })
        return { ...options.body, updated_at: '2026-10-16T00:00:00Z' }
      },
    }),
  }
  vm.createContext(context)
  vm.runInContext(ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText, context)
  const { updateAbout } = context.exports.useAdminPortfolio()

  const writable = {
    name: '',
    headline: 'A headline',
    intro: 'An introduction',
    profile_image: '',
    current_focus: 'A changed focus',
    skills: [{ group: 'Backend', items: ['Go'] }],
    timeline: [{ year: '2026', text: 'Verified work' }],
    contacts: [{ label: 'Email', href: 'mailto:reader@example.invalid', text: 'Contact' }],
    representative_links: [{ label: 'Source', href: 'https://example.com/source' }],
  }
  for (const content of [writable, {
    ...writable, headline: '', intro: '', current_focus: '',
    skills: [], timeline: [], contacts: [], representative_links: [],
  }]) {
    const profile = {
      ...structuredClone(content),
      id: '00000000-0000-7000-8000-000000000013',
      updated_at: '2026-10-15T17:28:40Z',
      unknown_response_metadata: 'not writable',
    }
    const before = structuredClone(profile)
    const reply = await updateAbout(profile)
    const { url, options } = requests.at(-1)
    assert.equal(url, '/api/v1/admin/about')
    assert.equal(options.method, 'PUT')
    assert.deepEqual(JSON.parse(JSON.stringify(options.body)), content)
    assert.deepEqual(profile, before, 'building the request must not mutate the editable profile')
    assert.equal(reply.updated_at, '2026-10-16T00:00:00Z')
  }
})
