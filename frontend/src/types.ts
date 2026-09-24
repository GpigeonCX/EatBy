export interface Location { id:string; name:string; sort_order:number }
export interface Product { id:string; name:string; category:string; default_unit:string; tracking_mode:'quantity'|'level'; low_threshold:number|null; after_open_days:number|null; barcode:string|null; favorite:boolean; total_quantity:number; batch_count:number; stock_state:string }
export interface Batch { id:string; product_id:string; location_id:string; quantity:number|null; level:string|null; expiry_date:string|null; opened_at:string|null; note:string; version:number; product_name:string; location_name:string; unit:string; effective_expiry:string|null; expiry_status:string }
export interface ShoppingItem { id:string; product_id:string|null; name:string; quantity:number|null; unit:string|null; checked:boolean }
export interface TodoItem { id:string; title:string; note:string; checked:boolean }
export interface Dashboard { expired:Batch[]; expiring:Batch[]; low_stock:Product[]; shopping:ShoppingItem[]; todos:TodoItem[]; locations:{id:string;name:string;count:number}[]; stock_count:number; warning_days:number; recent_event:{id:string;action:string;created_at:string}|null; generated_at:string }
