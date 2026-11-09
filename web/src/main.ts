import { VueQueryPlugin } from '@tanstack/vue-query'
import { createApp } from 'vue'

import App from './App.vue'
import { i18n } from './i18n'
import { queryClient } from './lib/query'
import router from './router'
import './styles/index.css'

createApp(App).use(router).use(i18n).use(VueQueryPlugin, { queryClient }).mount('#app')
