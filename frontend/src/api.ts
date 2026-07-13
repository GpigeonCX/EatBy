export class ApiError extends Error { constructor(public status:number, message:string){ super(message) } }
export async function api<T>(path:string, options:RequestInit={}):Promise<T>{
  const response=await fetch('/api/v1'+path,{credentials:'include',headers:{'Content-Type':'application/json',...(options.headers||{})},...options})
  const body=await response.json().catch(()=>({}))
  if(!response.ok) throw new ApiError(response.status,body.error||'请求失败')
  return body as T
}
export const post=<T>(path:string, body:unknown={})=>api<T>(path,{method:'POST',body:JSON.stringify(body)})
export const put=<T>(path:string, body:unknown)=>api<T>(path,{method:'PUT',body:JSON.stringify(body)})
export const del=<T>(path:string)=>api<T>(path,{method:'DELETE'})
