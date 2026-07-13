<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { api, ApiError, del, post, put } from './api'
import type { Batch, Dashboard, Location, Product, ShoppingItem } from './types'
import { expiryText, levelText, stockSummary } from './domain'
import BarcodeScanner from './components/BarcodeScanner.vue'

type Tab='home'|'stock'|'shopping'|'settings'
const loading=ref(true), initialized=ref(false), loggedIn=ref(false), error=ref(''), toast=ref('')
const tab=ref<Tab>('home'), me=ref<{name:string;role:string;household_name:string}|null>(null)
const dashboard=ref<Dashboard|null>(null), products=ref<Product[]>([]), batches=ref<Batch[]>([]), locations=ref<Location[]>([]), shopping=ref<ShoppingItem[]>([])
const showProduct=ref(false),showBatch=ref(false),showShopping=ref(false),showScanner=ref(false),showInviteJoin=ref(false)
const displayMode=location.pathname.startsWith('/display/'), displayToken=displayMode?decodeURIComponent(location.pathname.split('/').pop()||''):''
let displayTimer:number|undefined
const auth=reactive({family:'我的家',password:'',display_name:'',join_token:new URLSearchParams(location.search).get('token')||''})
const productForm=reactive<any>({id:'',name:'',category:'',default_unit:'件',tracking_mode:'quantity',low_threshold:1,after_open_days:null,barcode:null,favorite:false})
const batchForm=reactive<any>({id:'',product_id:'',location_id:'',quantity:1,level:'enough',expiry_date:null,opened_at:null,note:'',version:0})
const shoppingForm=reactive<any>({name:'',quantity:null,unit:'件'})
const settings=reactive({name:'',expiry_warning_days:7}),inviteLink=ref(''),displayLink=ref(''),sessions=ref<any[]>([]),displayTokens=ref<any[]>([])
const locationName=ref('')
const stockQuery=ref(''),locationFilter=ref('')
const filteredProducts=computed(()=>products.value.filter(p=>(!stockQuery.value||p.name.includes(stockQuery.value)||p.category.includes(stockQuery.value))&&(!locationFilter.value||batches.value.some(b=>b.product_id===p.id&&b.location_id===locationFilter.value))))
function notify(s:string){toast.value=s;window.setTimeout(()=>toast.value='',2200)}
function showError(e:unknown){error.value=e instanceof Error?e.message:'操作失败';window.setTimeout(()=>error.value='',3500)}
async function boot(){
  if(displayMode){await loadDisplay();loading.value=false;displayTimer=window.setInterval(loadDisplay,300000);return}
  try{const status=await api<{initialized:boolean}>('/status');initialized.value=status.initialized;if(auth.join_token){showInviteJoin.value=true;loading.value=false;return}if(status.initialized){await getMe()}}catch(e){if(!(e instanceof ApiError&&e.status===401))showError(e)}finally{loading.value=false}
}
async function getMe(){try{me.value=await api('/me');loggedIn.value=true;await loadAll()}catch{loggedIn.value=false}}
async function setup(){try{await post('/setup',{name:auth.family,password:auth.password,display_name:auth.display_name||'户主'});initialized.value=true;await getMe()}catch(e){showError(e)}}
async function login(){try{await post('/login',{password:auth.password,display_name:auth.display_name||'户主'});await getMe()}catch(e){showError(e)}}
async function join(){try{await post('/invites/accept',{token:auth.join_token,display_name:auth.display_name});history.replaceState({},'',location.pathname);showInviteJoin.value=false;await getMe()}catch(e){showError(e)}}
async function logout(){await post('/logout');loggedIn.value=false;me.value=null}
async function loadAll(){await Promise.all([loadDashboard(),loadProducts(),loadBatches(),loadLocations(),loadShopping()])}
async function loadDashboard(){dashboard.value=await api('/dashboard')}
async function loadProducts(){products.value=await api('/products/')}
async function loadBatches(){batches.value=await api('/batches/')}
async function loadLocations(){locations.value=await api('/locations/')}
async function loadShopping(){shopping.value=await api('/shopping/?all=1')}
async function loadDisplay(){try{dashboard.value=await api('/display/'+displayToken)}catch(e){showError(e)}}
function newProduct(barcode:string|null=null){Object.assign(productForm,{id:'',name:'',category:'',default_unit:'件',tracking_mode:'quantity',low_threshold:1,after_open_days:null,barcode,favorite:false});showProduct.value=true}
function editProduct(p:Product){Object.assign(productForm,p);showProduct.value=true}
async function saveProduct(){try{const body={...productForm,low_threshold:productForm.tracking_mode==='quantity'&&productForm.low_threshold!==''?Number(productForm.low_threshold):null,after_open_days:productForm.after_open_days===''?null:Number(productForm.after_open_days)||null};if(body.id)await put('/products/'+body.id,body);else await post('/products/',body);showProduct.value=false;await loadAll();notify('商品已保存')}catch(e){showError(e)}}
async function removeProduct(){if(!confirm('删除商品及其全部库存？'))return;try{await del('/products/'+productForm.id);showProduct.value=false;await loadAll()}catch(e){showError(e)}}
function newBatch(productId=''){const p=products.value.find(x=>x.id===productId);Object.assign(batchForm,{id:'',product_id:productId||products.value[0]?.id||'',location_id:locations.value[0]?.id||'',quantity:1,level:'enough',expiry_date:null,opened_at:null,note:'',version:0});if(p?.tracking_mode==='level')batchForm.quantity=null;showBatch.value=true}
function editBatch(b:Batch){Object.assign(batchForm,b);showBatch.value=true}
async function saveBatch(){try{const p=products.value.find(x=>x.id===batchForm.product_id);const body={...batchForm,quantity:p?.tracking_mode==='quantity'?Number(batchForm.quantity):null,level:p?.tracking_mode==='level'?batchForm.level:null,expiry_date:batchForm.expiry_date||null,opened_at:batchForm.opened_at||null};if(body.id)await put('/batches/'+body.id,body);else await post('/batches/',body);showBatch.value=false;await loadAll();notify('库存已保存')}catch(e){showError(e)}}
async function removeBatch(){if(!confirm('删除这批库存？'))return;try{await del(`/batches/${batchForm.id}?version=${batchForm.version}`);showBatch.value=false;await loadAll()}catch(e){showError(e)}}
async function adjust(b:Batch,delta?:number,level?:string){try{await post(`/batches/${b.id}/adjust`,{delta,level,version:b.version});await loadAll()}catch(e){showError(e)}}
async function openBatch(b:Batch){try{await post(`/batches/${b.id}/open`,{});await loadAll();notify('已标记开封')}catch(e){showError(e)}}
async function addToShopping(p:Product){try{await post('/shopping/from-product/'+p.id);await Promise.all([loadShopping(),loadDashboard()]);notify('已加入采购清单')}catch(e){showError(e)}}
async function addShopping(){try{await post('/shopping/',{...shoppingForm,quantity:shoppingForm.quantity?Number(shoppingForm.quantity):null});showShopping.value=false;shoppingForm.name='';shoppingForm.quantity=null;await loadShopping()}catch(e){showError(e)}}
async function toggleShopping(i:ShoppingItem){try{await put('/shopping/'+i.id,{...i,checked:!i.checked});await Promise.all([loadShopping(),loadDashboard()])}catch(e){showError(e)}}
async function removeShopping(i:ShoppingItem){await del('/shopping/'+i.id);await loadShopping()}
async function undo(){const event=dashboard.value?.recent_event;if(!event)return;try{await post('/events/'+event.id+'/undo');await loadAll();notify('已撤销最近操作')}catch(e){showError(e)}}
async function scanned(code:string){showScanner.value=false;try{const p=await api<Product>('/products/barcode/'+encodeURIComponent(code));newBatch(p.id)}catch(e){if(e instanceof ApiError&&e.status===404)newProduct(code);else showError(e)}}
async function openSettings(){tab.value='settings';try{const s=await api<any>('/settings');Object.assign(settings,s);if(me.value?.role==='owner'){sessions.value=await api('/sessions');displayTokens.value=await api('/display-tokens')}}catch(e){showError(e)}}
async function saveSettings(){try{await put('/settings',settings);if(me.value)me.value.household_name=settings.name;notify('设置已保存')}catch(e){showError(e)}}
async function createInvite(){try{const x=await post<any>('/invites');inviteLink.value=`${location.origin}/join?token=${x.token}`}catch(e){showError(e)}}
async function createDisplay(){try{const x=await post<any>('/display-tokens',{name:'Kindle'});displayLink.value=`${location.origin}/display/${x.token}`;displayTokens.value=await api('/display-tokens')}catch(e){showError(e)}}
async function copy(value:string){await navigator.clipboard.writeText(value);notify('链接已复制')}
async function addLocation(){if(!locationName.value.trim())return;try{await post('/locations/',{name:locationName.value,sort_order:locations.value.length});locationName.value='';await loadLocations()}catch(e){showError(e)}}
onMounted(boot);onBeforeUnmount(()=>displayTimer&&clearInterval(displayTimer))
</script>

<template>
  <div v-if="loading" class="center-page"><div class="loader"></div><p>正在打开家庭食材…</p></div>

  <main v-else-if="displayMode" class="kindle">
    <header><div><h1>家庭食材看板</h1><p>更新于 {{new Date(dashboard?.generated_at||'').toLocaleString('zh-CN',{hour12:false})}}</p></div><div class="kindle-count">库存 {{dashboard?.stock_count||0}} 批</div></header>
    <div v-if="error" class="kindle-error">{{error}} · 正在显示最后一次数据</div>
    <section class="kindle-grid">
      <article><h2>临期 / 已过期</h2><p v-if="!dashboard?.expired.length&&!dashboard?.expiring.length" class="kindle-empty">暂无临期食材</p><div v-for="b in [...(dashboard?.expired||[]),...(dashboard?.expiring||[])]" :key="b.id" class="kindle-row"><strong>{{b.product_name}}</strong><span>{{b.effective_expiry}} · {{b.location_name}}</span></div></article>
      <article><h2>快没了</h2><p v-if="!dashboard?.low_stock.length" class="kindle-empty">库存充足</p><div v-for="p in dashboard?.low_stock" :key="p.id" class="kindle-row"><strong>{{p.name}}</strong><span>{{p.tracking_mode==='quantity'?`${p.total_quantity} ${p.default_unit}`:levelText(p.stock_state)}}</span></div></article>
      <article><h2>采购清单</h2><p v-if="!dashboard?.shopping.length" class="kindle-empty">采购清单为空</p><div v-for="i in dashboard?.shopping" :key="i.id" class="kindle-row"><strong>□ {{i.name}}</strong><span>{{i.quantity?`${i.quantity} ${i.unit||''}`:''}}</span></div></article>
      <article><h2>存放位置</h2><div v-for="l in dashboard?.locations" :key="l.id" class="kindle-row"><strong>{{l.name}}</strong><span>{{l.count}} 批</span></div></article>
    </section>
  </main>

  <div v-else-if="showInviteJoin&&!loggedIn" class="auth-page"><section class="auth-card"><div class="brand-mark">食</div><h1>加入家庭库存</h1><p class="muted">输入你的称呼，这台设备之后会保持登录。</p><label>你的称呼<input v-model="auth.display_name" placeholder="例如：小林"></label><button class="primary wide" @click="join">加入家庭</button></section></div>

  <div v-else-if="!loggedIn" class="auth-page"><section class="auth-card"><div class="brand-mark">食</div><h1>{{initialized?'欢迎回来':'建立家庭食材库'}}</h1><p class="muted">{{initialized?'查看家里还有什么，别让食材悄悄过期。':'手机轻松管理，Kindle 随时一览。'}}</p><label v-if="!initialized">家庭名称<input v-model="auth.family" autocomplete="organization"></label><label>你的称呼<input v-model="auth.display_name" placeholder="户主"></label><label>管理密码<input v-model="auth.password" type="password" autocomplete="current-password" placeholder="至少 8 位"></label><button class="primary wide" @click="initialized?login():setup()">{{initialized?'登录':'开始使用'}}</button></section></div>

  <div v-else class="app-shell">
    <header class="topbar"><div><p class="eyebrow">{{me?.household_name}}</p><h1>{{tab==='home'?'今天家里有什么':tab==='stock'?'全部库存':tab==='shopping'?'采购清单':'家庭设置'}}</h1></div><button v-if="tab==='stock'" class="scan-button" @click="showScanner=true">▣ 扫码</button></header>

    <main class="content">
      <template v-if="tab==='home'">
        <section class="hero"><div><p>共记录</p><strong>{{dashboard?.stock_count||0}}</strong><span>批库存</span></div><button v-if="dashboard?.recent_event" class="ghost" @click="undo">↶ 撤销最近操作</button></section>
        <section v-if="dashboard?.expired.length" class="section"><div class="section-title"><h2>已经过期</h2><span class="badge danger">{{dashboard.expired.length}}</span></div><div class="cards"><button v-for="b in dashboard.expired" :key="b.id" class="item-card warning" @click="editBatch(b)"><div><strong>{{b.product_name}}</strong><p>{{b.location_name}} · {{expiryText(b)}}</p></div><span>处理 ›</span></button></div></section>
        <section class="section"><div class="section-title"><h2>{{dashboard?.warning_days}} 天内到期</h2><span class="badge">{{dashboard?.expiring.length||0}}</span></div><p v-if="!dashboard?.expiring.length" class="empty">近期没有要到期的食材，很好。</p><div class="cards"><button v-for="b in dashboard?.expiring" :key="b.id" class="item-card" @click="editBatch(b)"><div><strong>{{b.product_name}}</strong><p>{{b.location_name}} · {{expiryText(b)}}</p></div><span>›</span></button></div></section>
        <section class="section"><div class="section-title"><h2>快没了</h2><span class="badge">{{dashboard?.low_stock.length||0}}</span></div><p v-if="!dashboard?.low_stock.length" class="empty">当前库存看起来很充足。</p><div class="chips"><button v-for="p in dashboard?.low_stock" :key="p.id" @click="addToShopping(p)"><b>{{p.name}}</b><span>＋采购</span></button></div></section>
        <section class="section"><div class="section-title"><h2>按位置查看</h2></div><div class="location-grid"><button v-for="l in dashboard?.locations" :key="l.id" @click="tab='stock';locationFilter=l.id"><strong>{{l.name}}</strong><span>{{l.count}} 批</span></button></div></section>
      </template>

      <template v-else-if="tab==='stock'">
        <div class="toolbar"><input v-model="stockQuery" placeholder="搜索食材"><select v-model="locationFilter"><option value="">全部位置</option><option v-for="l in locations" :value="l.id">{{l.name}}</option></select></div>
        <p v-if="!filteredProducts.length" class="empty large">还没有食材。先添加一个常用商品，再记录库存。</p>
        <section v-for="p in filteredProducts" :key="p.id" class="product-block"><div class="product-head"><button @click="editProduct(p)"><strong>{{p.favorite?'★ ':''}}{{p.name}}</strong><span>{{p.category||'未分类'}} · {{stockSummary(p)}}</span></button><button class="small primary" @click="newBatch(p.id)">＋入库</button></div><div class="batch-row" v-for="b in batches.filter(x=>x.product_id===p.id)" :key="b.id"><button class="batch-main" @click="editBatch(b)"><span>{{b.location_name}}</span><b>{{b.quantity!==null?`${b.quantity} ${b.unit}`:levelText(b.level)}}</b><small :class="b.expiry_status">{{expiryText(b)}}</small></button><div class="quick"><button v-if="b.quantity!==null" @click="adjust(b,-1)">−1</button><button v-if="b.quantity!==null" @click="adjust(b,1)">＋1</button><button v-if="!b.opened_at" @click="openBatch(b)">开封</button></div></div></section>
        <button class="fab" @click="newProduct()">＋</button>
      </template>

      <template v-else-if="tab==='shopping'">
        <div class="shopping-list"><label v-for="i in shopping" :key="i.id" :class="{checked:i.checked}"><input type="checkbox" :checked="i.checked" @change="toggleShopping(i)"><span><strong>{{i.name}}</strong><small v-if="i.quantity">{{i.quantity}} {{i.unit}}</small></span><button @click.prevent="removeShopping(i)">×</button></label></div><p v-if="!shopping.length" class="empty large">采购清单是空的。</p><button class="fab" @click="showShopping=true">＋</button>
      </template>

      <template v-else>
        <section class="settings-card"><h2>家庭设置</h2><label>家庭名称<input v-model="settings.name"></label><label>提前提醒天数<input v-model.number="settings.expiry_warning_days" type="number" min="1" max="90"></label><button class="primary" @click="saveSettings">保存设置</button></section>
        <section class="settings-card"><h2>存放位置</h2><div class="tag-list"><span v-for="l in locations" :key="l.id">{{l.name}}</span></div><div class="inline-form"><input v-model="locationName" placeholder="新增位置"><button @click="addLocation">添加</button></div></section>
        <template v-if="me?.role==='owner'"><section class="settings-card"><h2>邀请家人</h2><p class="muted">邀请链接 24 小时内有效，只能使用一次。</p><button @click="createInvite">生成邀请链接</button><div v-if="inviteLink" class="link-box"><input :value="inviteLink" readonly><button @click="copy(inviteLink)">复制</button></div></section><section class="settings-card"><h2>Kindle 看板</h2><p class="muted">生成一个只能查看、不能修改数据的专用链接。</p><button @click="createDisplay">生成看板链接</button><div v-if="displayLink" class="link-box"><input :value="displayLink" readonly><button @click="copy(displayLink)">复制</button></div><p v-for="t in displayTokens" :key="t.id" class="device-row">{{t.name}}<button @click="del('/display-tokens/'+t.id).then(openSettings)">撤销</button></p></section><section class="settings-card"><h2>已登录设备</h2><p v-for="s in sessions" :key="s.id" class="device-row"><span>{{s.display_name}} · {{s.role==='owner'?'户主':'成员'}}</span><button v-if="s.id!==undefined" @click="del('/sessions/'+s.id).then(openSettings)">撤销</button></p></section></template>
        <button class="danger-link" @click="logout">退出当前设备</button>
      </template>
    </main>

    <nav class="bottom-nav"><button :class="{active:tab==='home'}" @click="tab='home';loadDashboard()"><span>⌂</span>首页</button><button :class="{active:tab==='stock'}" @click="tab='stock'"><span>▤</span>库存</button><button :class="{active:tab==='shopping'}" @click="tab='shopping'"><span>✓</span>采购</button><button :class="{active:tab==='settings'}" @click="openSettings"><span>⚙</span>设置</button></nav>
  </div>

  <div v-if="error&&!displayMode" class="toast error-toast">{{error}}</div><div v-if="toast" class="toast">{{toast}}</div>
  <BarcodeScanner v-if="showScanner" @found="scanned" @close="showScanner=false"/>

  <div v-if="showProduct" class="modal shade"><form class="modal-card" @submit.prevent="saveProduct"><div class="modal-head"><h3>{{productForm.id?'编辑商品':'添加商品'}}</h3><button type="button" class="icon" @click="showProduct=false">×</button></div><label>名称<input v-model="productForm.name" required autofocus></label><div class="form-grid"><label>分类<input v-model="productForm.category" placeholder="乳制品"></label><label>默认单位<input v-model="productForm.default_unit" placeholder="瓶"></label></div><label>记录方式<select v-model="productForm.tracking_mode"><option value="quantity">记录具体数量</option><option value="level">记录充足 / 快没了</option></select></label><label v-if="productForm.tracking_mode==='quantity'">低于多少算快没了<input v-model="productForm.low_threshold" type="number" step="0.1" min="0"></label><label>开封后建议天数<input v-model="productForm.after_open_days" type="number" min="1" placeholder="可不填"></label><label>商品条码<input v-model="productForm.barcode" placeholder="可扫码或手动填写"></label><label class="checkbox"><input v-model="productForm.favorite" type="checkbox">设为常用商品</label><button class="primary wide">保存商品</button><button v-if="productForm.id" type="button" class="danger-link" @click="removeProduct">删除商品</button></form></div>

  <div v-if="showBatch" class="modal shade"><form class="modal-card" @submit.prevent="saveBatch"><div class="modal-head"><h3>{{batchForm.id?'编辑库存':'添加库存'}}</h3><button type="button" class="icon" @click="showBatch=false">×</button></div><label>商品<select v-model="batchForm.product_id" required><option v-for="p in products" :value="p.id">{{p.name}}</option></select></label><label>存放位置<select v-model="batchForm.location_id" required><option v-for="l in locations" :value="l.id">{{l.name}}</option></select></label><label v-if="products.find(p=>p.id===batchForm.product_id)?.tracking_mode==='quantity'">数量<input v-model="batchForm.quantity" type="number" min="0" step="0.1" required></label><label v-else>当前状态<select v-model="batchForm.level"><option value="enough">充足</option><option value="half">一半</option><option value="low">快没了</option><option value="empty">用完</option></select></label><label>包装到期日<input v-model="batchForm.expiry_date" type="date"></label><label>备注<input v-model="batchForm.note" placeholder="例如：已放在上层"></label><button class="primary wide">保存库存</button><button v-if="batchForm.id" type="button" class="danger-link" @click="removeBatch">删除这批库存</button></form></div>
  <div v-if="showShopping" class="modal shade"><form class="modal-card" @submit.prevent="addShopping"><div class="modal-head"><h3>添加采购项</h3><button type="button" class="icon" @click="showShopping=false">×</button></div><label>名称<input v-model="shoppingForm.name" required autofocus></label><div class="form-grid"><label>数量<input v-model="shoppingForm.quantity" type="number" min="0" step="0.1"></label><label>单位<input v-model="shoppingForm.unit"></label></div><button class="primary wide">加入清单</button></form></div>
</template>
