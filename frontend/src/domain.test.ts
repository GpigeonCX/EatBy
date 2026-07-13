import { describe, expect, it } from 'vitest'
import { expiryText, levelText, stockSummary } from './domain'

describe('inventory presentation',()=>{
  it('renders qualitative stock levels',()=>{
    expect(levelText('low')).toBe('快没了')
    expect(levelText(null)).toBe('未记录')
  })
  it('renders quantity products with units',()=>{
    expect(stockSummary({tracking_mode:'quantity',total_quantity:2.5,default_unit:'盒'} as any)).toBe('2.5 盒')
  })
  it('marks an opened batch in its effective expiry label',()=>{
    expect(expiryText({effective_expiry:'2026-07-16',opened_at:'2026-07-13'} as any)).toBe('2026-07-16 · 已开封')
  })
})
