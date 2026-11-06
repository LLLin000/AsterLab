const assert = require('node:assert');
const dagre = require('../web/vendor/dagre.min.js');
const { layout } = require('../web/layout.js');

const node = (id, visual) => ({ node_id: id, component: { instance_id: id, visual } });
const edge = (from, to) => ({ from, to });
const overlap = (a, b) => a.x < b.x + 92 && a.x + 92 > b.x && a.y < b.y + 136 && a.y + 136 > b.y;

const graph = {
  nodes: [node('a'), node('b1'), node('b2'), node('merge'), node('read')],
  edges: [edge('a','b1'),edge('a','b2'),edge('b1','merge'),edge('b2','merge'),edge('merge','read')],
};
const pos = layout(graph.nodes, graph.edges, dagre);
assert.ok(pos.a.x < pos.b1.x && pos.a.x < pos.b2.x, 'branches follow source');
assert.ok(pos.b1.x < pos.merge.x && pos.b2.x < pos.merge.x, 'branches precede merge');
assert.ok(pos.merge.x < pos.read.x, 'merge precedes read');
assert.notDeepEqual(pos.b1, pos.b2, 'branches occupy separate slots');
for (const a of Object.values(pos)) for (const b of Object.values(pos)) if (a !== b) assert.ok(!overlap(a,b), 'no overlap');

const fixed = {
  nodes: [node('a'), node('b1', {x: 700, y: 40}), node('b2'), node('merge')],
  edges: [edge('a','b1'),edge('a','b2'),edge('b1','merge'),edge('b2','merge')],
};
const fixedPos = layout(fixed.nodes, fixed.edges, dagre);
assert.deepEqual(fixedPos.b1, {x: 700, y: 40}, 'manual position preserved');
for (const [id, p] of Object.entries(fixedPos)) if (id !== 'b1') assert.ok(!overlap(p, fixedPos.b1), 'auto node avoids manual node');

const nested = { nodes: [node('x'), node('y')], edges: [edge('x','y')] };
const nestedPos = layout(nested.nodes, nested.edges, dagre);
assert.ok(nestedPos.x.x < nestedPos.y.x, 'nested graph lays out independently');

console.log('w5 layout fixtures: ok');
