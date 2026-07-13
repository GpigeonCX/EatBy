import type { Batch, Product } from './types'

export function levelText(value:string|null|undefined):string {
  return ({ enough:'充足', half:'一半', low:'快没了', empty:'用完' } as Record<string,string>)[value||''] || '未记录'
}

export function stockSummary(product:Product):string {
  return product.tracking_mode==='quantity'
    ? `${product.total_quantity||0} ${product.default_unit}`
    : levelText(product.stock_state)
}

export function expiryText(batch:Batch):string {
  return batch.effective_expiry
    ? `${batch.effective_expiry}${batch.opened_at?' · 已开封':''}`
    : '未设置保质期'
}
