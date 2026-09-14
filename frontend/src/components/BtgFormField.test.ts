import { cleanup, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import BtgFormField from './BtgFormField.vue'
import BtgTextInput from './BtgTextInput.vue'

const FieldFixture = {
  components: { BtgFormField, BtgTextInput },
  props: ['description', 'error'],
  template: `
    <BtgFormField label="Email address" required :description="description" :error="error" v-slot="{ controlId, describedBy, invalid }">
      <BtgTextInput :id="controlId" :aria-describedby="describedBy" :invalid="invalid" type="email" />
    </BtgFormField>
  `,
}

describe('BtgFormField', () => {
  afterEach(cleanup)

  it('keeps a visible label associated with its native input', () => {
    render(FieldFixture, { props: { description: 'We use this only for account access.' } })
    const input = screen.getByRole('textbox', { name: /email address/i })
    expect(screen.getByText('Email address')).toBeTruthy()
    expect(input.getAttribute('aria-describedby')).toBeTruthy()
    expect(screen.getByText('We use this only for account access.').id).toBeTruthy()
  })

  it('associates an error and exposes invalid state without using colour alone', () => {
    render(FieldFixture, { props: { error: 'Enter a valid email address.' } })
    const input = screen.getByRole('textbox', { name: /email address/i })
    const error = screen.getByRole('alert')
    expect(input.getAttribute('aria-invalid')).toBe('true')
    expect(input.getAttribute('aria-describedby')).toContain(error.id)
  })

  it('remains keyboard focusable as a native input', () => {
    render(FieldFixture)
    const input = screen.getByRole('textbox', { name: /email address/i })
    input.focus()
    expect(document.activeElement).toBe(input)
  })

  it('generates distinct control IDs for separate fields', () => {
    render({
      components: { BtgFormField, BtgTextInput },
      template: `
        <BtgFormField label="Email" v-slot="{ controlId }"><BtgTextInput :id="controlId" /></BtgFormField>
        <BtgFormField label="Password" v-slot="{ controlId }"><BtgTextInput :id="controlId" type="password" /></BtgFormField>
      `,
    })

    expect((screen.getByLabelText('Email') as HTMLInputElement).id).not.toBe((screen.getByLabelText('Password') as HTMLInputElement).id)
  })
})
