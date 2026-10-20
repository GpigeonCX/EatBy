<script setup lang="ts">
import {ref} from 'vue'
import {post} from '../api'
const emit=defineEmits<{paired:[]}>()
const code=ref(''),name=ref('Kindle'),busy=ref(false),message=ref('')
async function pair(){busy.value=true;message.value='';try{await post('/kindle-pairings/claim',{code:code.value,name:name.value});message.value='绑定成功。Kindle 将自动获取配置；请确认设备显示正确家庭。';code.value='';emit('paired')}catch(e){message.value=e instanceof Error?e.message:'绑定失败'}finally{busy.value=false}}
</script>
<template><details><summary>使用配对码绑定 Kindle</summary><p>在 Kindle 的 KUAL 中选择 EatBy: pair device，输入屏幕上显示的配对码。只输入你自己设备上的码。</p><label>设备名称<input v-model="name" maxlength="60"></label><label>配对码<input v-model="code" placeholder="ABCD-EFGH" maxlength="9" autocapitalize="characters" autocomplete="off" spellcheck="false"></label><button :disabled="busy||code.length<8" @click="pair">确认绑定到当前家庭</button><p role="status">{{message}}</p></details></template>
