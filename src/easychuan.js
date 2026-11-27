import { Peer } from 'peerjs';

const chunkSize = 65_528;
const maxBufferedBytes = 4 * 1024 * 1024;
const peerOptions = {
  host: 'easychuan.cn',
  port: 9000,
  path: '/peer',
  secure: true,
  key: 'Vj5kd2EqWmE4OGE_KCxr',
  config: {
    iceServers: [
      { urls: 'stun:stun.miwifi.com' },
      { urls: 'stun:stun.chat.bilibili.com' },
      { urls: 'stun:turn.cloudflare.com:3478' },
    ],
  },
};

function indexFiles(registration, files) {
  if (typeof registration.fetch_code !== 'string' || !/^[a-z0-9]{1,32}$/i.test(registration.fetch_code)
      || !Array.isArray(registration.files)
      || registration.files.length !== files.length) {
    throw new Error('轻松传返回了无效的取件信息');
  }
  const indexed = new Map();
  const usedIndices = new Set();
  for (const { file_index: index, filekey: key } of registration.files) {
    if (!Number.isInteger(index) || !files[index] || usedIndices.has(index)
        || typeof key !== 'string' || !key || indexed.has(key)) {
      throw new Error('轻松传返回了无效的取件信息');
    }
    usedIndices.add(index);
    indexed.set(key, files[index]);
  }
  return indexed;
}

function waitForBuffer(channel, signal) {
  return new Promise((resolve, reject) => {
    if (signal.aborted || channel.readyState !== 'open') {
      reject(new Error('Data channel closed'));
      return;
    }
    const cleanup = () => {
      channel.removeEventListener('bufferedamountlow', onLow);
      signal.removeEventListener('abort', onAbort);
    };
    const onLow = () => { cleanup(); resolve(); };
    const onAbort = () => { cleanup(); reject(new Error('Data channel closed')); };
    channel.addEventListener('bufferedamountlow', onLow);
    signal.addEventListener('abort', onAbort);
    if (channel.bufferedAmount <= channel.bufferedAmountLowThreshold) onLow();
  });
}

async function sendChunks(transfer, channel, signal) {
  const { file, chunks } = transfer;
  // EasyChuan's raw data channel expects an 8-byte little-endian header.
  for (let index = 0; index < chunks; index++) {
    if (channel.bufferedAmount > maxBufferedBytes) await waitForBuffer(channel, signal);
    const offset = index * chunkSize;
    const bytes = await file.slice(offset, offset + chunkSize).arrayBuffer();
    if (signal.aborted || channel.readyState !== 'open') return;
    const packet = new ArrayBuffer(8 + bytes.byteLength);
    const header = new DataView(packet);
    header.setUint32(0, index, true);
    header.setUint32(4, chunks, true);
    new Uint8Array(packet, 8).set(new Uint8Array(bytes));
    channel.send(packet);
    transfer.queued++;
  }
}

async function connectionRoute(connection) {
  try {
    const pc = connection.peerConnection;
    const transport = pc.sctp?.transport
      ?? pc.getSenders?.().find(sender => sender.transport)?.transport;
    const selected = transport?.iceTransport?.getSelectedCandidatePair?.();
    let local = selected?.local?.type;
    let remote = selected?.remote?.type;
    if (!local || !remote) {
      const stats = await pc.getStats();
      const reports = [...stats.values()];
      const transport = reports.find(report => report.type === 'transport' && report.selectedCandidatePairId);
      const pair = transport
        ? stats.get(transport.selectedCandidatePairId)
        : reports.find(report => report.type === 'candidate-pair' && report.selected);
      local = stats.get(pair?.localCandidateId)?.candidateType;
      remote = stats.get(pair?.remoteCandidateId)?.candidateType;
    }
    if (local === 'relay' || remote === 'relay') return 'relay';
    if (local && remote) return 'direct';
  } catch { /* Connection statistics are optional across WebViews. */ }
  return 'unknown';
}

function openSender(id, onReady, onFailure) {
  const peer = new Peer(id, peerOptions);
  let stopped = false;
  const timeout = setTimeout(() => fail('连接轻松传超时，请重试'), 15_000);

  function stop() {
    if (stopped) return;
    stopped = true;
    clearTimeout(timeout);
    peer.destroy();
  }

  function fail(message) {
    stop();
    onFailure(message);
  }

  peer.on('open', () => {
    clearTimeout(timeout);
    if (!stopped) onReady();
  });
  peer.on('error', error => {
    if (!stopped && error.type !== 'peer-unavailable') fail('轻松传连接已断开，请重新生成取件码');
  });
  peer.on('disconnected', () => {
    if (!stopped) fail('轻松传连接已断开，请重新生成取件码');
  });
  return { peer, stop, isStopped: () => stopped };
}

export function startEasyChuanSender({ registration, files, onReady, onProgress, onFailure }) {
  const indexed = indexFiles(registration, files);
  const code = registration.fetch_code;
  const { peer, stop, isStopped } = openSender(`S-${code}`, () => onReady({ code, url: `https://easychuan.cn/r/${code}?t=nf` }), onFailure);
  let active = null;

  peer.on('connection', connection => {
    if (connection.serialization !== 'raw') {
      connection.close();
      return;
    }
    const lifetime = new AbortController();
    let current = null;
    let route = 'unknown';
    let finished = false;
    let receivingFailed = false;

    function report(phase) {
      if (isStopped() || active !== connection || !current) return;
      onProgress({ phase, route, filename: current.file.name, percent: Math.floor(current.received * 100 / current.chunks) });
    }

    function closed() {
      lifetime.abort();
      if (active !== connection) return;
      report(finished ? 'complete' : receivingFailed ? 'receiveFailed' : 'interrupted');
      active = null;
    }

    connection.on('open', () => {
      connection.dataChannel.bufferedAmountLowThreshold = 512 * 1024;
      connectionRoute(connection).then(result => {
        route = result;
        report('sending');
      });
    });
    connection.on('error', () => { closed(); connection.close(); });
    connection.on('close', closed);
    connection.on('data', data => {
      if (isStopped() || lifetime.signal.aborted) return;
      if (typeof data === 'string') {
        const file = indexed.get(data);
        if (!file || (active && active !== connection) || (current && current.queued !== current.chunks)) {
          connection.close();
          return;
        }
        active = connection;
        current = { file, chunks: Math.max(1, Math.ceil(file.size / chunkSize)), queued: 0, received: 0 };
        report('sending');
        sendChunks(current, connection.dataChannel, lifetime.signal).catch(() => connection.close());
        return;
      }
      if (active !== connection || !current || !(data instanceof ArrayBuffer)) return;
      if (data.byteLength === 4) {
        const count = new DataView(data).getUint32(0, true);
        if (count > current.received && count <= current.queued) {
          current.received = count;
          report('sending');
        }
      } else if (data.byteLength === 1) {
        const code = new Uint8Array(data)[0];
        if (code === 0 && current.queued === current.chunks) {
          current.received = current.chunks;
          finished = true;
          connection.close();
        } else if (code === 255) {
          receivingFailed = true;
          connection.close();
        }
      }
    });
  });

  return stop;
}

export function startEasyChuanMedia({ kind, registration, stream, onReady, onProgress, onFailure }) {
  const { fetch_code: code, stream_key: streamKey } = registration;
  if (typeof code !== 'string' || !/^[a-z0-9]{1,32}$/i.test(code)
      || typeof streamKey !== 'string' || !streamKey) {
    throw new Error('轻松传返回了无效的取件信息');
  }
  const screen = kind === 'screen';
  const prefix = screen ? 'SCN' : 'VID';
  const type = screen ? 's' : 'v';
  let active = null;
  const { peer, stop, isStopped } = openSender(
    `${prefix}-S-${code}`,
    () => onReady({ code, url: `https://easychuan.cn/r/${code}?t=${type}` }),
    error => {
      closeViewer();
      onFailure(error);
    },
  );

  function closeViewer() {
    if (!active) return;
    const { call, control, timer } = active;
    active = null;
    clearTimeout(timer);
    control?.close();
    call.close();
    if (!isStopped()) onProgress(null);
  }

  peer.on('call', call => {
    const metadata = call.metadata;
    if (active || isStopped() || metadata?.streamKey !== streamKey || !metadata.transactionId) {
      call.close();
      return;
    }
    const viewer = { call, transactionId: metadata.transactionId, control: null, timer: null };
    active = viewer;
    const close = () => { if (active === viewer) closeViewer(); };
    viewer.timer = setTimeout(close, 25_000);
    call.on('close', close);
    call.on('error', close);
    try {
      call.answer(stream);
      const pc = call.peerConnection;
      const update = async () => {
        if (active !== viewer || isStopped()) return;
        const state = pc.iceConnectionState;
        if (state === 'failed' || state === 'closed') {
          close();
          return;
        }
        if (state === 'disconnected' && viewer.timer === null) {
          viewer.timer = setTimeout(close, 25_000);
        }
        if (state !== 'connected' && state !== 'completed') return;
        clearTimeout(viewer.timer);
        viewer.timer = null;
        const route = await connectionRoute(call);
        if (active === viewer && !isStopped()) onProgress({ phase: 'viewing', route });
      };
      pc.addEventListener('iceconnectionstatechange', update);
      update();
    } catch {
      close();
    }
  });

  // The EasyChuan receiver opens a companion channel after it receives media.
  peer.on('connection', control => {
    const viewer = active;
    if (isStopped() || !viewer || viewer.control || control.peer !== viewer.call.peer
        || control.serialization !== 'raw'
        || control.metadata?.streamKey !== streamKey
        || control.metadata.transactionId !== viewer.transactionId) {
      control.close();
      return;
    }
    viewer.control = control;
    const close = () => { if (active === viewer) closeViewer(); };
    control.on('close', close);
    control.on('error', close);
  });

  return () => { stop(); closeViewer(); };
}
