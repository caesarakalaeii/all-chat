/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

// Real wire-format fixtures for the relay consumers. The canary and the
// fallback both decode base64 PushFrames with the connector's own schemas,
// so a test frame that only looks like a PushFrame proves nothing: it must
// decode to a real message with decodedData, exactly like a relay frame.
//
// Two quirks shape this helper:
//  - the generated proto crashes on any unset int64 field ("Cannot convert
//    undefined to a BigInt") and on a missing routeParams map, so every
//    message starts from decode(Buffer.alloc(0)) (full defaults) and then
//    patches the fields it cares about;
//  - tiktok-live-proto's exports map has no `require` condition, so the
//    module must load via dynamic ESM import — hence async fixture makers.
//    It is a transitive dependency the service intentionally does not
//    import in product code (see index.ts); tests may.

type ProtoCodec = {
  decode: (input: Uint8Array) => Record<string, unknown>;
  encode: (message: unknown) => { finish: () => Uint8Array };
};
type ProtoModule = Record<string, ProtoCodec>;

export type ChatFrame = { base64: string; comment: string; handle: string };

let protoPromise: Promise<ProtoModule> | undefined;

async function loadProto(): Promise<ProtoModule> {
  if (!protoPromise) {
    // moduleResolution "node" cannot resolve the package's exports map;
    // runtime resolves fine (see self.ts for the product-code precedent of
    // living with this tsconfig's proto limitations).
    // @ts-expect-error tiktok-live-proto/v3 is not resolvable under this tsconfig
    protoPromise = import('tiktok-live-proto/v3') as unknown as Promise<ProtoModule>;
  }
  return protoPromise;
}

/** One PushFrame carrying one WebcastChatMessage, plus the values it encodes. */
export async function makeChatFrame(comment = 'hello', handle = 'viewer1'): Promise<ChatFrame> {
  const proto = await loadProto();
  const common = {
    ...proto.CommonMessageData.decode(Buffer.alloc(0)),
    msgId: '1',
    method: 'WebcastChatMessage'
  };
  const user = {
    ...proto.User.decode(Buffer.alloc(0)),
    displayId: handle,
    nickname: 'Viewer One'
  };
  const chat = proto.WebcastChatMessage.encode({
    ...proto.WebcastChatMessage.decode(Buffer.alloc(0)),
    comment,
    user,
    common
  }).finish();
  const message = {
    method: 'WebcastChatMessage',
    payload: chat,
    msgId: '0',
    offset: '0',
    msgType: 0,
    isHistory: false
  };
  const fetchResult = {
    ...proto.ProtoMessageFetchResult.decode(Buffer.alloc(0)),
    cursor: 'abc',
    messages: [message]
  };
  const frame = proto.WebcastPushFrame.encode({
    ...proto.WebcastPushFrame.decode(Buffer.alloc(0)),
    payloadEncoding: 'pb',
    payloadType: 'msg',
    payload: proto.ProtoMessageFetchResult.encode(fetchResult).finish()
  }).finish();
  return { base64: Buffer.from(frame).toString('base64'), comment, handle };
}

/** A PushFrame with no inner messages — the ack/keepalive shape (~50% of relay frames). */
export async function makeAckFrame(): Promise<string> {
  const proto = await loadProto();
  const fetchResult = {
    ...proto.ProtoMessageFetchResult.decode(Buffer.alloc(0)),
    cursor: 'abc',
    messages: []
  };
  const frame = proto.WebcastPushFrame.encode({
    ...proto.WebcastPushFrame.decode(Buffer.alloc(0)),
    payloadEncoding: 'pb',
    payloadType: 'msg',
    payload: proto.ProtoMessageFetchResult.encode(fetchResult).finish()
  }).finish();
  return Buffer.from(frame).toString('base64');
}
