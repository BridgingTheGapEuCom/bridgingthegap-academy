<script lang="ts">
import { defineComponent, h, type PropType, type VNode } from 'vue'
import type { RichTextInline } from '../lesson/content'

function renderInline(inline: RichTextInline): VNode {
  if (inline.type === 'hard_break') return h('br')

  let node: VNode = h('span', inline.text)
  for (const mark of inline.marks) {
    if (mark.type === 'strong') node = h('strong', [node])
    else if (mark.type === 'emphasis') node = h('em', [node])
    else if (mark.type === 'inline_code') node = h('code', [node])
    else if (mark.type === 'link') node = h('a', { href: mark.href }, [node])
  }
  return node
}

export default defineComponent({
  name: 'LessonRichTextInlines',
  props: { items: { type: Array as PropType<RichTextInline[]>, required: true } },
  setup(props) {
    return () => props.items.map((inline) => renderInline(inline))
  },
})
</script>
