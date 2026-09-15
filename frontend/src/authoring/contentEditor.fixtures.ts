import type { CanonicalBlock } from './contentEditor'

export const deferredBlocks: CanonicalBlock[] = [
  { key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'diagram' }, decorative: false, altText: 'Architecture', caption: 'Published diagram' } },
  { key: 'video', type: 'VIDEO', payload: { asset: { assetKey: 'video' }, title: 'Demo', transcript: 'Transcript', captionsAsset: { assetKey: 'captions' } } },
  { key: 'audio', type: 'AUDIO', payload: { asset: { assetKey: 'audio' }, title: 'Interview', transcriptAsset: { assetKey: 'transcript' } } },
  { key: 'table', type: 'TABLE', payload: { caption: 'Comparison', headers: ['Method'], rows: [['Async']] } },
  { key: 'download', type: 'DOWNLOAD', payload: { asset: { assetKey: 'notes' }, label: 'Notes', description: 'Reference' } },
  { key: 'check', type: 'KNOWLEDGE_CHECK', payload: { assessmentKey: 'check-one' } },
]
