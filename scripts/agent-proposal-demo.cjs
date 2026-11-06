#!/usr/bin/env node
// End-to-end W6 proposal demo using only the public HTTP API.
// Usage: node scripts/agent-proposal-demo.cjs <view-id> [--accept]
const assert = require('node:assert');
const base = process.env.ASTERLAB_URL || 'http://127.0.0.1:19000';
const viewId = process.argv[2];
const accept = process.argv.includes('--accept');
if (!viewId) {
  console.error('usage: node scripts/agent-proposal-demo.cjs <view-id> [--accept]');
  process.exit(2);
}

async function json(path, options) {
  const response = await fetch(`${base}${path}`, options);
  const body = await response.json();
  return { response, body };
}
async function revision() {
  const { body } = await json(`/api/views/${viewId}`);
  return body.latest_version.data.revision;
}
async function propose(changeset) {
  return json(`/api/views/${viewId}/propose`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ changeset }),
  });
}

(async () => {
  const view = await json(`/api/views/${viewId}`);
  assert.equal(view.response.ok, true, 'view read failed');
  const doc = view.body.latest_version.data;
  const selection = await json(`/api/views/${viewId}/selection/concentration_table`);
  assert.equal(selection.response.ok, true, 'selection context failed');
  console.log('selected:', selection.body.renderer_key, 'revision:', doc.revision);

  const changeset = {
    changeset_id: `demo-${Date.now()}`,
    document_id: doc.document_id,
    expected_revision: doc.revision,
    operations: [{
      kind: 'move_slot_child',
      instance_id: 'concentration_table',
      target_parent_instance_id: 'step',
      target_slot_id: 'detail',
      target_index: 0,
    }],
    provenance: ['agent-proposal-demo'],
  };
  const preview = await propose(changeset);
  assert.equal(preview.response.ok, true, 'proposal preview failed');
  assert.equal(preview.body.valid, true);
  console.log('preview revision:', preview.body.new_revision, 'changes:', preview.body.operation_changes);

  if (!accept) {
    const after = await revision();
    assert.equal(after, doc.revision, 'reject/preview must not write');
    console.log('preview only; document unchanged at revision', after);
    return;
  }

  const commit = await fetch(`${base}/api/views/${viewId}/revise`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ changeset }),
  });
  const commitBody = await commit.json();
  assert.equal(commit.ok, true, `commit failed: ${JSON.stringify(commitBody)}`);
  const next = await revision();
  assert.equal(next, doc.revision + 1, 'accepted proposal must increment revision');
  console.log('accepted; revision', next);

  const stale = await propose({ ...changeset, changeset_id: `stale-${Date.now()}` });
  assert.equal(stale.response.ok, false, 'stale proposal must be rejected');
  console.log('stale proposal rejected:', stale.body.error || stale.body);
})().catch(error => { console.error(error); process.exit(1); });
