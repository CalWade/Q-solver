import test from 'node:test'
import assert from 'node:assert/strict'
import { assessMatrix } from './compatibility.mjs'

const fixture = () => ({ schemaVersion: 1, cases: [{ id: 'fixture', platform: 'macOS', product: 'Local synthetic receiver', scenario: 'share-display', expectedResult: 'excluded', result: 'not_run' }] })
test('a structurally valid untested matrix cannot claim completion', () => {
  assert.deepEqual(assessMatrix(fixture()), { total: 1, recorded: 0, notRun: 1, structurallyValid: true, complete: false, verifiedCompatible: false, errors: [] })
})
test('capture results require versioned evidence and an independent receiver', () => {
  const matrix = fixture(); matrix.cases[0].result = 'excluded'
  assert.equal(assessMatrix(matrix).structurallyValid, false)
  Object.assign(matrix.cases[0], { osVersion: 'fixture OS', productVersion: 'fixture receiver', appRevision: 'test-only', observedAt: '2026-11-08', evidence: ['synthetic-unit-fixture'], independentReceiver: true })
  assert.equal(assessMatrix(matrix).complete, true)
  matrix.cases[0].result = 'visible'
  assert.equal(assessMatrix(matrix).complete, true)
  assert.equal(assessMatrix(matrix).verifiedCompatible, false)
  matrix.cases[0].result = 'excluded'
  matrix.cases[0].scenario = 'keyboard'
  assert.equal(assessMatrix(matrix).structurallyValid, false)
})
test('duplicate cases and inconclusive observations cannot inflate coverage', () => {
  const matrix = fixture(); matrix.cases.push({ ...matrix.cases[0] })
  assert.equal(assessMatrix(matrix).structurallyValid, false)
  matrix.cases = [{ ...matrix.cases[0], result: 'inconclusive' }]
  assert.equal(assessMatrix(matrix).complete, false)
})
