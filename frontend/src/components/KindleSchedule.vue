<script setup lang="ts">
import { ref } from 'vue'
import { api, put } from '../api'
const props=defineProps<{id:string}>()
type Window={days:number[];start_minute:number;end_minute:number;interval_minutes:number}
type Schedule={show_status:boolean;orientation:'portrait'|'landscape_cw'|'landscape_ccw';timezone:string;enabled:boolean;windows:Window[]}
type Response={schedule:Schedule;next_refresh_at:string|null}
const schedule=ref<Schedule|null>(null),message=ref(''),busy=ref(false),next=ref<string|null>(null)
const days=['一','二','三','四','五','六','日']
async function load(){busy.value=true;message.value='';try{const x=await api<Response>(`/display-tokens/${props.id}/schedule`);schedule.value=x.schedule;next.value=x.next_refresh_at}catch(e){message.value=String(e)}finally{busy.value=false}}
async function save(){busy.value=true;message.value='';try{const x=await put<Response>(`/display-tokens/${props.id}/schedule`,schedule.value);schedule.value=x.schedule;next.value=x.next_refresh_at;message.value='已保存，设备下次连接时生效。'}catch(e){message.value=String(e)}finally{busy.value=false}}
function clock(n:number){return `${String(Math.floor(n/60)).padStart(2,'0')}:${String(n%60).padStart(2,'0')}`}
function minutes(event:Event){const value=(event.target as HTMLInputElement).value;const [h,m]=value.split(':').map(Number);return (h??0)*60+(m??0)}
function toggle(w:Window,d:number){w.days=w.days.includes(d)?w.days.filter(x=>x!==d):[...w.days,d].sort()}
</script>
<template>
  <div class="schedule-editor">
    <button v-if="!schedule" :disabled="busy" @click="load">配置刷新时间</button>
    <template v-else>
      <label class="checkbox"><input v-model="schedule.show_status" type="checkbox">底部显示更新时间和电量（刷新时采样）</label>
      <label>显示方向<select v-model="schedule.orientation"><option value="portrait">竖屏 · 600×800</option><option value="landscape_cw">横屏 · 800×600 · 图片顺时针旋转</option><option value="landscape_ccw">横屏 · 800×600 · 图片逆时针旋转</option></select></label>
      <p>横屏会重新排版，不拉伸竖屏内容；选择适合支架和充电口位置的方向。不改变 Kindle 系统方向。</p>
      <label class="checkbox"><input v-model="schedule.enabled" type="checkbox">启用定时刷新</label>
      <p>北京时间 · 周一至周五不含法定调休判断 · 结束时间不刷新</p>
      <fieldset v-for="(w,index) in schedule.windows" :key="index">
        <legend>时间段 {{ index+1 }}</legend>
        <div class="weekdays"><button v-for="(day,i) in days" :key="i" :aria-pressed="w.days.includes(i+1)" @click="toggle(w,i+1)">周{{day}}</button></div>
        <label>开始<input type="time" :value="clock(w.start_minute)" @change="w.start_minute=minutes($event)"></label>
        <label>结束<select :value="w.end_minute===1440?'midnight':'custom'" @change="w.end_minute=($event.target as HTMLSelectElement).value==='midnight'?1440:1380"><option value="midnight">24:00</option><option value="custom">指定时间</option></select><input v-if="w.end_minute!==1440" type="time" :value="clock(w.end_minute)" @change="w.end_minute=minutes($event)"></label>
        <label>间隔（分钟）<input v-model.number="w.interval_minutes" type="number" min="1" max="1440"></label>
        <button @click="schedule.windows.splice(index,1)">删除时间段</button>
      </fieldset>
      <button :disabled="schedule.windows.length>=32" @click="schedule.windows.push({days:[1,2,3,4,5],start_minute:1080,end_minute:1440,interval_minutes:30})">添加时间段</button>
      <button class="primary" :disabled="busy" @click="save">保存计划</button>
      <p>已保存计划的下一时刻：{{next?new Date(next).toLocaleString('zh-CN',{timeZone:'Asia/Shanghai',hour12:false}):'已暂停'}}</p>
      <p>设备休眠时无法立即接收修改；可在 Kindle 手动同步。此设置供设备客户端使用，不改变网页当前的五分钟刷新。</p>
    </template>
    <p role="status">{{message}}</p>
  </div>
</template>
<style scoped>
.schedule-editor{padding:12px 0;font-size:14px}.schedule-editor p{line-height:1.6}.weekdays{display:flex;flex-wrap:wrap;gap:5px}.weekdays button[aria-pressed=true]{background:#245b45;color:white}fieldset{border:1px solid #ccc;border-radius:10px;margin:12px 0;padding:12px}.schedule-editor>button{margin:4px}input[type=checkbox]{width:20px}
</style>
