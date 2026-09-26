#!/usr/bin/env node
// Independent interoperability check using Digital Bazaar's JavaScript
// eddsa-rdfc-2022 implementation. It intentionally does not import Academy
// signing or verification code and never uses a network document loader.
import {readFileSync} from 'node:fs';
import {gunzipSync} from 'node:zlib';
import jsigs from 'jsonld-signatures';
import {DataIntegrityProof} from '@digitalbazaar/data-integrity';
import {cryptosuite} from '@digitalbazaar/eddsa-rdfc-2022-cryptosuite';
import dataIntegrityContext from '@digitalbazaar/data-integrity-context';

const [fixturePath] = process.argv.slice(2);
if(!fixturePath) {
  throw new Error('usage: openbadges-interop.mjs fixture.json');
}
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8'));
const vcContext = JSON.parse(readFileSync(new URL(
  '../internal/modules/credentials/openbadges/signing/contexts/vc-v2.jsonld',
  import.meta.url), 'utf8'));
const obContext = JSON.parse(readFileSync(new URL(
  '../internal/modules/credentials/openbadges/signing/contexts/ob3-3.0.3.jsonld',
  import.meta.url), 'utf8'));

const documents = new Map([
  ['https://www.w3.org/ns/credentials/v2', vcContext],
  ['https://purl.imsglobal.org/spec/ob/v3p0/context-3.0.3.json', obContext],
  ...dataIntegrityContext.contexts,
  [fixture.issuer.id, fixture.issuer],
]);
for(const method of fixture.issuer.verificationMethod) {
  documents.set(method.id, method);
}
const documentLoader = async url => {
  const document = documents.get(url);
  if(!document) {
    throw new Error(`network document loading is forbidden: ${url}`);
  }
  return {contextUrl: null, documentUrl: url, document};
};

const suite = new DataIntegrityProof({cryptosuite});
const purpose = new jsigs.purposes.AssertionProofPurpose({
  controller: {
    id: fixture.issuer.id,
    assertionMethod: fixture.issuer.assertionMethod,
  },
});
async function verify(document, label) {
  const result = await jsigs.verify(document, {suite, purpose, documentLoader});
  if(!result.verified) {
    throw new Error(`${label} did not verify: ${result.error}\n${JSON.stringify(result.error?.errors ?? result.error, null, 2)}`);
  }
}

const {badge, statusList, expected} = fixture;
if(!Array.isArray(badge['@context']) || !badge['@context'].includes('https://www.w3.org/ns/credentials/v2') || !badge.type.includes('OpenBadgeCredential')) {
  throw new Error('badge does not have the expected OB3/VC structure');
}
const entry = badge.credentialStatus;
if(entry.type !== 'BitstringStatusListEntry' || entry.statusPurpose !== 'revocation' || !/^(0|[1-9][0-9]*)$/.test(entry.statusListIndex) || entry.statusListCredential !== statusList.id) {
  throw new Error('badge status entry is not a valid revocation reference');
}
if(!statusList.type.includes('BitstringStatusListCredential') || statusList.credentialSubject.statusPurpose !== 'revocation') {
  throw new Error('status list is not a revocation Bitstring Status List credential');
}
await verify(badge, 'Open Badge credential');
await verify(statusList, 'Bitstring Status List credential');

const encoded = statusList.credentialSubject.encodedList;
if(typeof encoded !== 'string' || !encoded.startsWith('u')) {
  throw new Error('status list encoding is not multibase base64url');
}
const compressed = Buffer.from(encoded.slice(1), 'base64url');
const bits = gunzipSync(compressed);
const index = Number(entry.statusListIndex);
const revoked = (bits[Math.floor(index / 8)] & (1 << (7 - (index % 8)))) !== 0;
if(revoked !== expected.revoked) {
  throw new Error(`status bit mismatch: expected revoked=${expected.revoked}`);
}
if(fixture.issuer.verificationMethod.some(method => /secret|private|seed/i.test(JSON.stringify(method)))) {
  throw new Error('issuer representation includes secret material');
}
console.log('Digital Bazaar eddsa-rdfc-2022 interoperability: verified');
