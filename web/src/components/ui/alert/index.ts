import { cva, type VariantProps } from 'class-variance-authority'

export { default as Alert } from './Alert.vue'
export { default as AlertDescription } from './AlertDescription.vue'
export { default as AlertTitle } from './AlertTitle.vue'

export const alertVariants = cva('relative w-full rounded-lg border px-4 py-3 text-sm', {
  variants: {
    variant: {
      default: 'bg-card text-card-foreground',
      destructive: 'border-destructive/40 bg-destructive/5 text-destructive',
      success: 'border-emerald-500/40 bg-emerald-50 text-emerald-900',
    },
  },
  defaultVariants: {
    variant: 'default',
  },
})

export type AlertVariants = VariantProps<typeof alertVariants>
