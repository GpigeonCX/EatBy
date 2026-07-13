<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { BrowserMultiFormatReader, type IScannerControls } from '@zxing/browser'
const emit=defineEmits<{found:[value:string],close:[]}>()
const error=ref('')
const reader=new BrowserMultiFormatReader()
let controls:IScannerControls|undefined
onMounted(async()=>{try{controls=await reader.decodeFromVideoDevice(undefined,'scanner-video',(result)=>{if(result){controls?.stop();emit('found',result.getText())}})}catch(e){error.value='无法打开摄像头，请检查 HTTPS 和相机权限'}})
onBeforeUnmount(()=>{controls?.stop()})
</script>
<template><div class="modal shade"><div class="modal-card scanner"><div class="modal-head"><h3>扫描商品条码</h3><button class="icon" @click="$emit('close')">×</button></div><video id="scanner-video"></video><p v-if="error" class="error">{{error}}</p><p class="muted">将包装条码放入取景框内</p></div></div></template>
