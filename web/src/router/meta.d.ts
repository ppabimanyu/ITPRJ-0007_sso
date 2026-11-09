import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    titleKey?: string
    requiresAuth?: boolean
    requiresAdmin?: boolean
  }
}

export {}
