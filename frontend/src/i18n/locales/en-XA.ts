import englishMessages from './en'
import { pseudoLocalizeMessages } from '../pseudo'

export default pseudoLocalizeMessages(englishMessages) as typeof englishMessages
