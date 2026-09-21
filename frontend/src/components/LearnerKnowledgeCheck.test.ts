import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import LearnerKnowledgeCheck from './LearnerKnowledgeCheck.vue'
import type { PublishedAssessmentLearnerView } from '../courses/courses'

const assessment: PublishedAssessmentLearnerView = {
  assessmentKey: '10000000-0000-4000-8000-000000000001',
  questions: [
    { stableKey: 'single', type: 'SINGLE_CHOICE', prompt: 'Choose one', position: 0, options: [{ stableKey: 'first', text: 'First choice', position: 0 }, { stableKey: 'second', text: 'Second choice', position: 1 }], leftItems: [], rightItems: [] },
    { stableKey: 'multiple', type: 'MULTIPLE_CHOICE', prompt: 'Choose several', position: 1, options: [{ stableKey: 'one', text: 'One', position: 0 }, { stableKey: 'two', text: 'Two', position: 1 }], leftItems: [], rightItems: [] },
    { stableKey: 'matching', type: 'MATCHING', prompt: 'Match these', position: 2, options: [], leftItems: [{ stableKey: 'left-one', text: 'Left one', position: 0 }, { stableKey: 'left-two', text: 'Left two', position: 1 }], rightItems: [{ stableKey: 'right-one', text: 'Right one', position: 0 }, { stableKey: 'right-two', text: 'Right two', position: 1 }] },
  ],
}

describe('LearnerKnowledgeCheck', () => {
  afterEach(cleanup)

  it('renders native answer controls in frozen question and option order', async () => {
    render(LearnerKnowledgeCheck, { props: { assessment } })
    expect(screen.getAllByRole('group').map((element) => element.querySelector('legend')?.textContent)).toEqual(['1. Choose one', '2. Choose several', '3. Match these'])
    const radios = screen.getAllByRole('radio') as HTMLInputElement[]
    await fireEvent.click(radios[1])
    expect(radios[1].checked).toBe(true)
    const checkboxes = screen.getAllByRole('checkbox') as HTMLInputElement[]
    await fireEvent.click(checkboxes[0])
    await fireEvent.click(checkboxes[1])
    expect(checkboxes.map((element) => element.checked)).toEqual([true, true])
    expect(screen.getAllByRole('combobox')).toHaveLength(2)
    expect(screen.queryByRole('button', { name: /submit|check answer|grade/i })).toBeNull()
  })

  it('keeps answer keys and correctness data out of the rendered DOM', () => {
    const { container } = render(LearnerKnowledgeCheck, { props: { assessment } })
    expect(container.innerHTML).not.toMatch(/correctOption|correctPairs|answerKey|answers/i)
    expect(screen.getByText(/not saved or graded yet/i)).toBeTruthy()
  })
})
