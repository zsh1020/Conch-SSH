import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'

// ── 设计系统字体 IBM Plex Sans / Mono：@font-face 见 style.css（本地 woff2，离线）──
// ── Phosphor 图标（自托管，仅 woff2；避免打包 svg/ttf/woff 冗余格式，省 ~11MB）──
import './assets/phosphor/regular/style.css'
import './assets/phosphor/fill/style.css'
import './assets/phosphor/bold/style.css'

import './style.css'

const app = createApp(App)
app.use(createPinia())
app.mount('#app')
