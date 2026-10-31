import { defineStore } from 'pinia'
import { ref } from 'vue'

export type DialogKind = 'info' | 'error' | 'confirm' | 'prompt'

/** 控制 prompt 弹窗打开时的光标/选区行为
 *  'end'  — 光标置于末尾（默认，用于新建文件/目录等空输入场景）
 *  'name' — 仅选中文件名部分（不含最后一个 . 后的扩展名），用于重命名文件
 *  'all'  — 全选（原有行为）
 */
export type SelectMode = 'end' | 'name' | 'all'

export interface DialogItem {
  id:           number
  kind:         DialogKind
  title:        string
  message:      string
  placeholder:  string
  defaultValue: string
  selectMode:   SelectMode
  /** prompt 输入是否掩码显示（密码场景） */
  secret:       boolean
  resolve:      (value: boolean | string | null) => void
}

export const useDialogStore = defineStore('dialogs', () => {
  const stack = ref<DialogItem[]>([])
  let seq = 1

  function _push(
    kind:         DialogKind,
    title:        string,
    message:      string,
    placeholder:  string,
    defaultValue: string,
    selectMode:   SelectMode = 'end',
    secret:       boolean = false,
  ): Promise<boolean | string | null> {
    return new Promise(resolve => {
      stack.value.push({ id: seq++, kind, title, message, placeholder, defaultValue, selectMode, secret, resolve })
    })
  }

  function close(id: number, value: boolean | string | null) {
    const idx = stack.value.findIndex(d => d.id === id)
    if (idx !== -1) {
      stack.value[idx].resolve(value)
      stack.value.splice(idx, 1)
    }
  }

  /** 普通提示，只有"好的"按钮 */
  const info = (message: string, title = '提示') =>
    _push('info', title, message, '', '') as Promise<null>

  /** 错误提示，红色主题，只有"好的"按钮 */
  const error = (message: string, title = '操作失败') =>
    _push('error', title, message, '', '') as Promise<null>

  /** 确认对话框，返回 true/false */
  const confirm = (message: string, title = '确认') =>
    _push('confirm', title, message, '', '') as Promise<boolean>

  /** 输入对话框，返回字符串或 null（取消）
   *  selectMode: 打开时的选区行为（默认 'end' 光标末尾，'name' 仅选文件名，'all' 全选）
   */
  const prompt = (message: string, title = '输入', placeholder = '', defaultValue = '', selectMode: SelectMode = 'end') =>
    _push('prompt', title, message, placeholder, defaultValue, selectMode) as Promise<string | null>

  /** 密码输入对话框，输入掩码显示，返回字符串或 null（取消） */
  const password = (message: string, title = '输入密码', placeholder = '') =>
    _push('prompt', title, message, placeholder, '', 'end', true) as Promise<string | null>

  return { stack, close, info, error, confirm, prompt, password }
})
