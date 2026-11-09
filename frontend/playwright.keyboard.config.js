import base from './playwright.native.config.js'

export default { ...base, testMatch: 'keyboard.spec.js', outputDir: 'test-results/native-keyboard' }
