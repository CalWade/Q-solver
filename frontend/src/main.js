import './styles/global.css'
import 'katex/dist/katex.min.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'

async function start() {
  if (import.meta.env.DEV && import.meta.env.MODE === 'harness') {
    const { installHarnessBridge } = await import('./services/harness-bridge')
    installHarnessBridge()
  }
  const app = createApp(App)
  app.use(createPinia())
  app.mount('#app')
}
start()
