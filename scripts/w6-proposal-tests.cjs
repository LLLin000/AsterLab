#!/usr/bin/env node
// W6 proposal acceptance/rejection/staleness regression against a running server.
// Usage: node scripts/w6-proposal-tests.cjs <view-id>
const assert = require('node:assert');
const base = process.env.ASTERLAB_URL || 'http://127.0.0.1:19000';
const viewId = process.argv[2];
if (!viewId) throw new Error('usage: node scripts/w6-proposal-tests.cjs <view-id>');

async function call(path, options) {
  const response = await fetch(`${base}${path}`, options);
  return { response, body: await response.json() };
}
async function current() {
  const { body } = await call(`/api/views/${viewId}`);
  return body.latest_version.data;
}
function changeset(doc, operations) {
  return { changeset_id: `test-${Date.now()}-${Math.random()}`, document_id: doc.document_id, expected_revision: doc.revision, operations, provenance: ['w6-test'] };
}
async function propose(cs) {
  return call(`/api/views/${viewId}/propose`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ changeset: cs }) });
}
async function revise(cs) {
  return call(`/api/views/${viewId}/revise`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ changeset: cs }) });
}

(async () => {
  let doc = await current();
  const start = doc.revision;
  const table = 'concentration_table';

  const noop = await propose(changeset(doc, [{ kind: 'move_slot_child', instance_id: table, target_parent_instance_id: 'step', target_slot_id: 'detail', target_index: 0 }]));
  assert.equal(noop.response.ok, true);
  assert.equal(noop.body.no_op, true, 'move to same index must be a no-op');
  assert.equal((await current()).revision, start, 'no-op must not write');

  const add = changeset(doc, [{ kind: 'add_slot_child', parent_instance_id: 'step', slot_id: 'detail', child: { instance_id: `test-note-${Date.now()}`, definition: { id: 'content.note', version: 1 }, props: { text: 'proposal test' }, slots: {} } }]);
  const addPreview = await propose(add);
  assert.equal(addPreview.response.ok, true);
  assert.equal((await current()).revision, start, 'preview must not write');

  const accepted = await revise(add);
  assert.equal(accepted.response.ok, true, 'add should commit');
  doc = await current();
  assert.equal(doc.revision, start + 1);
  const note = add.operations[0].child.instance_id;

  const remove = changeset(doc, [{ kind: 'remove_instance', instance_id: note }]);
  const removed = await revise(remove);
  assert.equal(removed.response.ok, true, 'remove should commit');
  doc = await current();
  assert.equal(doc.revision, start + 2);

  const stale = changeset(doc, [{ kind: 'update_props', instance_id: note, property: 'text', value: 'stale' }]);
  const changed = changeset(doc, [{ kind: 'update_props', instance_id: 'note', property: 'text', value: 'revision moved' }]);
  assert.equal((await revise(changed)).response.ok, true);
  const staleResult = await revise(stale);
  assert.equal(staleResult.response.ok, false, 'stale proposal must be rejected');
  assert.match(JSON.stringify(staleResult.body), /revision/i);

  console.log('w6 proposal tests: ok');
})().catch(error => { console.error(error); process.exit(1); });
