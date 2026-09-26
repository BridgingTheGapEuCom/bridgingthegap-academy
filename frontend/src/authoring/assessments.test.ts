import { describe, expect, it } from 'vitest'
import { moveInOrder, newQuestion, normalizeAssessmentQuestions } from './assessments'

describe('Assessment editor helpers', () => {
  it('creates stable keys once and keeps them while presentation order changes', () => {
    const first = newQuestion('SINGLE_CHOICE')
    const second = newQuestion('MULTIPLE_CHOICE')
    const moved = normalizeAssessmentQuestions(moveInOrder([first, second], 0, 1))
    expect(moved.map((question) => question.stableKey)).toEqual([second.stableKey, first.stableKey])
    expect(moved.map((question) => question.position)).toEqual([0, 1])
    expect(new Set(first.options.map((option) => option.stableKey)).size).toBe(2)
  })

  it('creates deterministic closed question shapes for all supported types', () => {
    const single = newQuestion('SINGLE_CHOICE')
    const multiple = newQuestion('MULTIPLE_CHOICE')
    const matching = newQuestion('MATCHING')
    expect(single.correctOptionKeys).toHaveLength(1)
    expect(multiple.correctOptionKeys).toHaveLength(1)
    expect(matching.correctPairs).toHaveLength(2)
    expect(new Set(matching.correctPairs.map((pair) => pair.rightKey)).size).toBe(2)
  })
})
