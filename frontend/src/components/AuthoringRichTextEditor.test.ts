import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import AuthoringRichTextEditor from './AuthoringRichTextEditor.vue'

describe('AuthoringRichTextEditor', () => {
  afterEach(cleanup)

  it('shows existing formatting in one semantic WYSIWYG surface', async () => {
    render(AuthoringRichTextEditor, { props: { label: 'Lesson text', content: { nodes: [
      { type: 'paragraph', content: [
        { type: 'text', text: 'Strong', marks: [{ type: 'strong' }] },
        { type: 'text', text: ' emphasis', marks: [{ type: 'emphasis' }] },
        { type: 'text', text: ' code', marks: [{ type: 'inline_code' }] },
      ] },
      { type: 'bullet_list', items: [[{ type: 'text', text: 'Bullet', marks: [] }]] },
    ] } } })
    const editor = await screen.findByRole('textbox', { name: 'Lesson text' })
    expect(editor.querySelector('strong')?.textContent).toBe('Strong')
    expect(editor.querySelector('em')?.textContent).toBe(' emphasis')
    expect(editor.querySelector('code')?.textContent).toBe(' code')
    expect(editor.querySelector('ul li')?.textContent).toBe('Bullet')
    for (const name of ['Bold', 'Italic', 'Link', 'Bulleted list', 'Numbered list', 'Inline code']) expect(screen.getByRole('button', { name })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Bold' }).getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByRole('toolbar', { name: 'Lesson text formatting' })).toBeTruthy()
  })

  it('presents an empty writing surface without persisting placeholder text', async () => {
    const { emitted } = render(AuthoringRichTextEditor, { props: { label: 'Lesson text', content: { nodes: [{ type: 'paragraph', content: [] }] } } })
    const editor = await screen.findByRole('textbox', { name: 'Lesson text' })
    expect(editor.closest('.authoring-rich-text__frame')?.classList.contains('is-empty')).toBe(true)
    expect(emitted()['update:content']).toBeUndefined()
  })

  it('exposes active formatting and a non-modal link popover', async () => {
    render(AuthoringRichTextEditor, { props: { label: 'Lesson text', content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Bold', marks: [{ type: 'strong' }] }] }] } } })
    const editor = await screen.findByRole('textbox', { name: 'Lesson text' })
    await fireEvent.focus(editor)
    expect(screen.getByRole('button', { name: 'Bold' }).getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(screen.getByRole('button', { name: 'Link' }))
    expect(screen.getByRole('group', { name: 'Link editor' })).toBeTruthy()
    expect(screen.queryByRole('dialog')).toBeNull()
    await fireEvent.keyDown(screen.getByRole('textbox', { name: 'Link URL' }), { key: 'Escape' })
    expect(screen.queryByRole('group', { name: 'Link editor' })).toBeNull()
  })

  it('creates a multiline code block separately from inline code', async () => {
    const { emitted } = render(AuthoringRichTextEditor, { props: { label: 'Lesson text', codeBlocks: true, content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'kubectl get pods', marks: [] }] }] } } })
    expect(screen.getByRole('button', { name: 'Inline code' })).toBeTruthy()
    const codeBlock = screen.getByRole('button', { name: 'Code block' })
    expect(codeBlock.getAttribute('title')).toBe('Code block')
    await fireEvent.click(codeBlock)
    const editor = screen.getByRole('textbox', { name: 'Lesson text' })
    expect(editor.querySelector('pre > code')?.textContent).toBe('kubectl get pods')
    expect(codeBlock.getAttribute('aria-pressed')).toBe('true')
    const updates = emitted()['update:content'] as unknown as Array<[{ nodes: unknown[] }]>
    expect(updates.at(-1)?.[0].nodes[0]).toEqual({ type: 'code_block', text: 'kubectl get pods' })
  })

  it('normalizes browser content to the constrained canonical model', async () => {
    const { emitted } = render(AuthoringRichTextEditor, { props: { label: 'Lesson text', content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Before', marks: [] }] }] } } })
    const editor = await screen.findByRole('textbox', { name: 'Lesson text' })
    editor.innerHTML = '<p style="color:red" onclick="alert(1)"><strong>Safe</strong> <a href="javascript:alert(1)">unsafe link</a></p><ul><li>Item</li></ul><script>alert(1)</script>'
    await fireEvent.input(editor)
    const updates = emitted()['update:content'] as unknown as Array<[unknown]>
    const content = updates.at(-1)![0]
    expect(content).toEqual({ nodes: [
      { type: 'paragraph', content: [{ type: 'text', text: 'Safe', marks: [{ type: 'strong' }] }, { type: 'text', text: ' unsafe link', marks: [] }] },
      { type: 'bullet_list', items: [[{ type: 'text', text: 'Item', marks: [] }]] },
    ] })
    expect(JSON.stringify(content)).not.toMatch(/javascript|onclick|color|script/i)
  })

  it('rejects dangerous link URLs in the accessible link control', async () => {
    render(AuthoringRichTextEditor, { props: { label: 'Lesson text', content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Link me', marks: [] }] }] } } })
    await fireEvent.click(screen.getByRole('button', { name: 'Link' }))
    await fireEvent.update(screen.getByRole('textbox', { name: 'Link URL' }), 'javascript:alert(1)')
    await fireEvent.click(screen.getByRole('button', { name: 'Apply link' }))
    expect(screen.getByRole('alert').textContent).toContain('safe HTTPS or internal URL')
  })
})
