import { VueQueryPlugin } from '@tanstack/vue-query'
import { createApp } from 'vue'

import App from './App.vue'
import { i18n } from './i18n'
import { clearAuthCache } from './lib/auth-cache'
import { setSessionExpiredHandler } from './lib/http'
import { queryClient } from './lib/query'
import router from './router'
import './styles/index.css'

setSessionExpiredHandler(() => {
  clearAuthCache(queryClient)
  const current = router.currentRoute.value
  if (current.name === 'login') {
    return
  }
  void router.replace({
    name: 'login',
    query: current.fullPath === '/' ? {} : { redirect: current.fullPath },
  })
})

createApp(App).use(router).use(i18n).use(VueQueryPlugin, { queryClient }).mount('#app')
