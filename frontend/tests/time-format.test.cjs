const assert = require('node:assert/strict')
const { execFileSync } = require('node:child_process')
const path = require('node:path')
const { test } = require('node:test')
const frontend = path.resolve(__dirname, '..')
const webFile = path.join(frontend, 'app/composables/useArticles.ts')
const miniFile = path.resolve(frontend, '../miniprogram/miniprogram/utils/format.ts')
const probe = `
const fs=require('node:fs'),vm=require('node:vm'),ts=require('typescript')
function load(file){const exports={};vm.runInNewContext(ts.transpileModule(fs.readFileSync(file,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{exports});return exports}
const web=load(${JSON.stringify(webFile)}),mini=load(${JSON.stringify(miniFile)})
const values=['2026-11-02T15:59:59Z','2026-11-02T16:00:00Z','2026-11-03T00:00:00+08:00','2026-12-31T16:00:00Z','2028-02-28T16:00:00Z']
console.log(JSON.stringify({web:values.map(web.formatDate),mini:values.map(mini.formatDate),iso:web.articleDate({seconds:'1793635200'}),invalid:[web.formatDate('bad-date'),mini.formatDate('bad-date'),web.formatDate(),mini.formatDate()]}))
`
for (const timezone of ['UTC', 'Asia/Shanghai', 'America/Los_Angeles']) {
  test(`Web and Mini keep Shanghai day boundaries under ${timezone}`, () => {
    const result = JSON.parse(execFileSync(process.execPath, ['-e', probe], { cwd: frontend, env: { ...process.env, TZ: timezone }, encoding: 'utf8' }))
    const expected = ['2026-11-02', '2026-11-03', '2026-11-03', '2027-01-01', '2028-02-29']
    assert.deepEqual(result.web.map(value => value.replaceAll('/', '-')), expected)
    assert.deepEqual(result.mini, expected)
    assert.equal(result.iso, '2026-11-02T16:00:00.000Z')
    assert.deepEqual(result.invalid, ['', '', '', ''])
  })
}
