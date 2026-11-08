import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

export function assessMatrix(matrix) {
  const errors = [], ids = new Set()
  const supported = new Set(['not_run', 'excluded', 'visible', 'blank', 'focus_changed', 'focus_retained', 'input_leaked', 'input_blocked', 'inconclusive'])
  const cases = matrix.cases || []
  if (matrix.schemaVersion !== 1 || cases.length === 0) errors.push('matrix schema/cases missing')
  let recorded = 0
  for (const item of cases) {
    if (!item.id || ids.has(item.id)) errors.push(`missing/duplicate case ID: ${item.id}`)
    ids.add(item.id)
    if (!item.platform || !item.product || !item.scenario || !supported.has(item.result) || !supported.has(item.expectedResult) || ['not_run', 'inconclusive'].includes(item.expectedResult)) errors.push(`invalid case: ${item.id}`)
    if (item.result === 'not_run') continue
    recorded++
    for (const key of ['osVersion', 'productVersion', 'appRevision', 'observedAt']) if (!item[key]) errors.push(`${item.id}: missing ${key}`)
    if (!Array.isArray(item.evidence) || !item.evidence.length) errors.push(`${item.id}: missing evidence`)
    if (item.scenario.startsWith('share-') && !item.independentReceiver) errors.push(`${item.id}: remote result requires an independent receiver`)
    if (item.result === 'excluded' && !item.scenario.startsWith('share-')) errors.push(`${item.id}: capture exclusion cannot be inferred from focus/input tests`)
  }
  const complete = cases.length > 0 && recorded === cases.length && errors.length === 0 && cases.every(item => item.result !== 'inconclusive')
  return { total: cases.length, recorded, notRun: cases.length - recorded, structurallyValid: errors.length === 0, complete, verifiedCompatible: complete && cases.every(item => item.result === item.expectedResult), errors }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const matrix = JSON.parse(await readFile(new URL('../docs/compatibility/matrix.json', import.meta.url), 'utf8'))
  const result = assessMatrix(matrix)
  console.log(JSON.stringify(result, null, 2))
  if (!result.structurallyValid || process.argv.includes('--require-complete') && !result.verifiedCompatible) process.exitCode = 1
}
