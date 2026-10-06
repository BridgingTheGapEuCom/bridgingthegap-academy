import type { CanonicalBlock } from './contentEditor'
import type { BtgIconName } from '../components/btg-icon-names'

export interface ContentBlockPresentation {
  icon: BtgIconName
  labelKey: `authoring.content.blocks.${CanonicalBlock['type']}.label`
}

const presentations: Record<CanonicalBlock['type'], ContentBlockPresentation> = {
  TEXT: { icon: 'text', labelKey: 'authoring.content.blocks.TEXT.label' },
  HEADING: { icon: 'text', labelKey: 'authoring.content.blocks.HEADING.label' },
  IMAGE: { icon: 'image', labelKey: 'authoring.content.blocks.IMAGE.label' },
  VIDEO: { icon: 'video', labelKey: 'authoring.content.blocks.VIDEO.label' },
  AUDIO: { icon: 'audio', labelKey: 'authoring.content.blocks.AUDIO.label' },
  DOWNLOAD: { icon: 'document', labelKey: 'authoring.content.blocks.DOWNLOAD.label' },
  CODE: { icon: 'code', labelKey: 'authoring.content.blocks.CODE.label' },
  CALLOUT: { icon: 'callout', labelKey: 'authoring.content.blocks.CALLOUT.label' },
  QUOTE: { icon: 'document', labelKey: 'authoring.content.blocks.QUOTE.label' },
  DIVIDER: { icon: 'divider', labelKey: 'authoring.content.blocks.DIVIDER.label' },
  KNOWLEDGE_CHECK: { icon: 'objective', labelKey: 'authoring.content.blocks.KNOWLEDGE_CHECK.label' },
  PLUGIN_WIDGET: { icon: 'settings', labelKey: 'authoring.content.blocks.PLUGIN_WIDGET.label' },
  TABLE: { icon: 'content', labelKey: 'authoring.content.blocks.TABLE.label' },
}

export function contentBlockPresentation(type: CanonicalBlock['type']): ContentBlockPresentation {
  return presentations[type]
}
