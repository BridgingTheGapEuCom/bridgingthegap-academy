import type { AuthoringAssetAttachmentCopy } from '../components/AuthoringAssetAttachment.vue'

export const authoringAssetAttachmentTestCopy: AuthoringAssetAttachmentCopy = {
  attachmentLabel: (label) => `${label} attachment`,
  attachedAssetFallback: 'Attached asset',
  change: 'Change audio', choose: 'Choose audio', pickerTitle: 'Change audio', pickerDescription: 'Choose an existing audio asset or upload a new one.',
  uploadLabel: 'Upload new audio', uploadDescription: 'Choose a file to upload for this audio block.', selected: (filename) => `Selected: ${filename}`,
  uploadAndUse: 'Upload and use', uploading: 'Uploading…', existingAssets: 'Existing audio assets', loadingAssets: 'Loading uploaded assets…', loadUnavailable: 'Assets are unavailable.', retry: 'Retry', noAssets: 'No uploaded assets yet.', availableAssets: 'Available uploaded assets', useAsset: (filename) => `Use ${filename}`, previous: 'Previous', next: 'Next', cancel: 'Cancel', incompatibleUpload: 'The uploaded file is incompatible.', uploadedAttached: (filename) => `${filename} uploaded.`, assetAttached: (filename) => `${filename} attached.`, uploadInvalid: 'Upload failed.', uploadTooLarge: 'Upload is too large.', sessionEnded: 'Session ended.', uploadUnavailable: 'Upload unavailable.',
}
