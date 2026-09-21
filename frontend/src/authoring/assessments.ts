import type { AuthoringAssessmentQuestion, AuthoringAssessmentMatchingItem, AuthoringAssessmentOption } from './authoring'

export type AssessmentQuestionType = AuthoringAssessmentQuestion['type']

// Keys are stable semantic identities. Positions are presentation order and
// are recomputed after an author moves an item.
export function newAssessmentKey(prefix: 'question' | 'option' | 'left' | 'right'): string {
  return `${prefix}-${crypto.randomUUID()}`
}

export function newQuestion(type: AssessmentQuestionType): AuthoringAssessmentQuestion {
  const stableKey = newAssessmentKey('question')
  if (type === 'MATCHING') {
    const leftItems = [newMatchingItem('left', 0, 'First item'), newMatchingItem('left', 1, 'Second item')]
    const rightItems = [newMatchingItem('right', 0, 'First match'), newMatchingItem('right', 1, 'Second match')]
    return { stableKey, type, prompt: 'New matching question', position: 0, options: [], correctOptionKeys: [], leftItems, rightItems, correctPairs: leftItems.map((left, index) => ({ leftKey: left.stableKey, rightKey: rightItems[index]!.stableKey })) }
  }
  const options = [newOption(0, 'First option'), newOption(1, 'Second option')]
  return { stableKey, type, prompt: 'New question', position: 0, options, correctOptionKeys: [options[0]!.stableKey], leftItems: [], rightItems: [], correctPairs: [] }
}

export function newOption(position: number, text = ''): AuthoringAssessmentOption { return { stableKey: newAssessmentKey('option'), text, position } }
export function newMatchingItem(side: 'left' | 'right', position: number, text = ''): AuthoringAssessmentMatchingItem { return { stableKey: newAssessmentKey(side), text, position } }

export function normalizeAssessmentQuestions(questions: AuthoringAssessmentQuestion[]): AuthoringAssessmentQuestion[] {
  return questions.map((question, position) => ({
    ...question,
    position,
    options: question.options.map((option, optionPosition) => ({ ...option, position: optionPosition })),
    leftItems: question.leftItems.map((item, itemPosition) => ({ ...item, position: itemPosition })),
    rightItems: question.rightItems.map((item, itemPosition) => ({ ...item, position: itemPosition })),
  }))
}

export function moveInOrder<T>(items: T[], index: number, direction: -1 | 1): T[] {
  const destination = index + direction
  if (destination < 0 || destination >= items.length) return items
  const result = [...items]
  const [item] = result.splice(index, 1)
  result.splice(destination, 0, item!)
  return result
}

export function assessmentFingerprint(value: { title: string; questions: AuthoringAssessmentQuestion[] }): string {
  return JSON.stringify(value)
}
