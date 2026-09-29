// @nuxt/eslint 在 `nuxt prepare` 时生成 .nuxt/eslint.config.mjs，这里在其之上叠加项目规则
import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt()
