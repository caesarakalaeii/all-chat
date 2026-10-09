import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createRelay } from './server.mjs';

async function withRelay(fetchImpl, fn) {
  const server = createRelay({ token: 'secret', fetchImpl }).listen(0);
  await new Promise((r) => server.once('listening', r));
  try {
    await fn(`http://127.0.0.1:${server.address().port}`);
  } finally {
    server.close();
  }
}

const upstream = (body) => async () => ({ json: async () => body });
const auth = { 'x-relay-token': 'secret' };

test('rejects a missing or wrong token without calling TikTok', async () => {
  let called = false;
  await withRelay(async () => { called = true; }, async (base) => {
    assert.equal((await fetch(`${base}/v1/live?handle=soyabontv`)).status, 401);
    assert.equal((await fetch(`${base}/v1/live?handle=soyabontv`, { headers: { 'x-relay-token': 'nope' } })).status, 401);
  });
  assert.equal(called, false);
});

test('only forwards well-formed handles, lowercased', async () => {
  const urls = [];
  const fetchImpl = async (url) => { urls.push(url); return { json: async () => ({ statusCode: 19881007, message: 'user_not_found' }) }; };
  await withRelay(fetchImpl, async (base) => {
    for (const bad of ['', 'a', 'x/../y', 'a%26b=c', 'way_too_long_for_a_tiktok_handle']) {
      assert.equal((await fetch(`${base}/v1/live?handle=${bad}`, { headers: auth })).status, 400, bad);
    }
    assert.equal((await fetch(`${base}/v1/live?handle=SoyabonTV`, { headers: auth })).status, 200);
  });
  assert.equal(urls.length, 1);
  assert.match(urls[0], /uniqueId=soyabontv&/);
});

test('reports a live account with its room id', async () => {
  const body = { statusCode: 0, data: { user: { id: '686', roomId: '769' }, liveRoom: { status: 2 } } };
  await withRelay(upstream(body), async (base) => {
    const res = await (await fetch(`${base}/v1/live?handle=soyabontv`, { headers: auth })).json();
    assert.deepEqual(res, { found: true, live: true, status: 2, roomId: '769', userId: '686' });
  });
});

test('reports an unknown handle as not found', async () => {
  await withRelay(upstream({ statusCode: 19881007, message: 'user_not_found' }), async (base) => {
    const res = await (await fetch(`${base}/v1/live?handle=nobody_here`, { headers: auth })).json();
    assert.deepEqual(res, { found: false, code: 19881007, message: 'user_not_found' });
  });
});
