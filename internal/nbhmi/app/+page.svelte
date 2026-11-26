<script lang="ts">
	import { onMount } from 'svelte';
	import { RealtimeClient } from '@joyautomation/nautilus-hmi';
	import imported from '$lib/nb-hmi.json';
	import Screen from '$lib/Screen.svelte';
	import { allows, displayNumber, isPassword, parseNumber, type HMIModel, type Part, type Register, type ScreenModel, type Value } from '$lib/model';

	const doc = imported as HMIModel;
	const windows = new Map(doc.windows.map((screen) => [screen.id, screen]));
	const login = doc.windows.find((screen) => /login/i.test(screen.name));
	const fileList = doc.windows.find((screen) => screen.parts.some((p) => p.kind === 'FileList'));
	const confirmation = doc.windows.find((screen) => /confirm action/i.test(screen.name));
	const client = new RealtimeClient<{ tags: Record<string, Value>; quality?: Record<string, string> }>({ tags: ['NB_CIO_*'] });

	let active = $state(doc.start);
	let overlay = $state<number>();
	let registers = $state<Record<string, Value>>(
		Object.fromEntries(Array.from({ length: 10 }, (_, i) => ['LB' + (9000 + i), true])),
	);
	let files = $state<string[]>([]);
	let editing = $state.raw<Part>();
	let draft = $state('');
	let replaceOnKey = $state(false);
	let pending = $state<Part>();
	let token = $state('');
	let tokenEntry = $state('');
	let busy = $state(false);
	let error = $state('');
	let outerWidth = $state(doc.width);
	const scale = $derived(Math.min(1, outerWidth / doc.width));
	const primary = $derived(windows.get(active));
	const common = windows.get(doc.common);
	const selected = $derived(overlay === undefined ? undefined : windows.get(overlay));
	const direct = $derived(directWindows(primary, common, selected));

	onMount(() => {
		client.start();
		return () => client.stop();
	});

	function read(r?: Register): Value {
		if (editing && r?.area === 'LW') {
			if (r.index === 9060 || r.index === 9070) return isPassword(editing) ? '*'.repeat(draft.length) : draft;
			if (editing.kind === 'Number Input' && (r.index === 9180 || r.index === 9190)) {
				const [low, high] = limits(editing);
				return formatLimit(editing, r.index === 9180 ? high : low);
			}
		}
		return stored(r);
	}

	function stored(r?: Register): Value {
		if (!r) return 0;
		if (r.area === 'CIO') return client.frame?.tags?.[r.key] ?? false;
		return registers[r.key] ?? (r.area === 'LB' ? false : 0);
	}

	function limits(p: Part): [number, number] {
		const low = p.low ? Number(stored(p.low)) : p.parameters?.[0] ?? 0;
		const high = p.high ? Number(stored(p.high)) : p.parameters?.[1] ?? 65535;
		return [low, high];
	}

	function formatLimit(p: Part, raw: number): string {
		return isPassword(p) ? String(raw) : displayNumber(p, raw);
	}

	function editValue(p: Part): string | undefined {
		if (editing === p && p.kind === 'Number Input' && !isPassword(p)) return draft;
	}

	function canWrite(r?: Register): boolean {
		return r?.area !== 'CIO' || (
			!busy && client.connected && r.key in (client.frame?.tags ?? {}) &&
			!client.frame?.quality?.[r.key]
		);
	}

	function canRequestLogin(p: Part): boolean {
		return !!login && !!p.autoLogin && !!p.enabled?.login && !token && allows(p.enabled, read, true);
	}

	function canActivate(p: Part): boolean {
		return canRequestLogin(p) || (allows(p.enabled, read, !!token) && canWrite(p.write));
	}

	function directWindows(...roots: (ScreenModel | undefined)[]): ScreenModel[] {
		const visible: ScreenModel[] = [];
		const seen = new Set(roots.filter((root) => root !== undefined).map((root) => root.id));
		for (const root of roots) {
			visit(root);
		}
		return visible;

		function visit(screen?: ScreenModel) {
			for (const p of screen?.parts ?? []) {
				if (p.kind !== 'Direct Window' || p.target === undefined || seen.has(p.target) ||
					!allows(p.visible, read, !!token) || Boolean(read(p.read)) !== (p.mode !== 1)) continue;
				const child = windows.get(p.target);
				if (!child) continue;
				seen.add(child.id);
				visible.push({ ...child, bounds: { ...child.bounds, x: p.bounds.x, y: p.bounds.y } });
				visit(child);
			}
		}
	}

	function centered(screen: ScreenModel): ScreenModel {
		const x = Math.max(0, (doc.width - screen.bounds.w) / 2);
		const y = Math.max(0, (doc.height - screen.bounds.h) / 2);
		return { ...screen, bounds: { ...screen.bounds, x, y } };
	}

	function leave(screen?: ScreenModel) {
		for (const p of screen?.parts ?? []) {
			if (p.kind === 'Set Bit' && p.write?.area === 'LB' && (p.mode === 6 || p.mode === 7)) {
				registers[p.write.key] = p.mode === 6;
			}
		}
	}

	function closeOverlay() {
		leave(selected);
		overlay = undefined;
		editing = undefined;
		pending = undefined;
		tokenEntry = '';
		error = '';
	}

	function openOverlay(target: number) {
		closeOverlay();
		overlay = target;
	}

	function changeWindow(target?: number) {
		if (target === undefined || !windows.has(target)) return;
		leave(primary);
		closeOverlay();
		active = target;
	}

	async function write(r: Register | undefined, value: Value): Promise<boolean> {
		if (!r) return false;
		if (r.area !== 'CIO') {
			registers[r.key] = value;
			return true;
		}
		if (!canWrite(r)) return false;
		busy = true;
		try {
			const refusal = await client.writeTag(r.key, value, { token });
			if (refusal) {
				error = refusal;
				return false;
			}
			return true;
		} finally {
			busy = false;
		}
	}

	function notify(p: Part) {
		const key = p.meta?.notify;
		if (key) registers[key] = key.startsWith('LB') ? p.meta?.notifyValue === '1' : Number(p.meta?.notifyValue ?? 0);
	}

	async function activate(p: Part, confirmed = false) {
		error = '';
		if (login && canRequestLogin(p)) {
			openOverlay(login.id);
			return;
		}
		if (!allows(p.enabled, read, !!token) || !canWrite(p.write)) return;
		if (p.confirm && !confirmed) {
			pending = p;
			if (confirmation) overlay = confirmation.id;
			return;
		}
		switch (p.kind) {
		case 'Function Key':
			if (p.function === 'Input char') {
				await typeKey(p.char ?? '');
			} else if (p.meta?.navigation === 'Change window') {
				changeWindow(p.target);
			} else if (p.meta?.navigation === 'Close window') {
				closeOverlay();
			}
			break;
		case 'Switch':
		case 'Set Bit': {
			const mode = p.mode ?? 0;
			if (p.write?.key === 'LB9165' && login) {
				openOverlay(login.id);
				return;
			}
			if (p.write?.key === 'LB9166') {
				token = '';
				closeOverlay();
				return;
			}
			if (mode <= 2 && await write(p.write, mode === 2 ? !Boolean(read(p.read ?? p.write)) : mode === 0)) notify(p);
			break;
		}
		case 'Word Set': {
			const word = p.parameters?.[0] ?? 0;
			if (await write(p.write, word)) notify(p);
			if (pending) {
				await finishConfirm(word === 1);
			}
			break;
		}
		case 'Number Input':
		case 'Text Input':
			editing = p;
			replaceOnKey = p.kind === 'Number Input';
			draft = p.kind === 'Number Input'
				? (isPassword(p) ? '' : displayNumber(p, read(p.read)))
				: String(read(p.read));
			if (p.kind === 'Text Input' && draft === '0') draft = '';
			overlay = p.keyboard === 65535 ? undefined : p.keyboard;
			break;
		}
	}

	async function finishConfirm(ok: boolean) {
		const action = pending;
		closeOverlay();
		if (ok && action) await activate(action, true);
	}

	async function finishInput() {
		if (!editing) return;
		const p = editing;
		let value: Value = draft;
		if (p.kind === 'Number Input') {
			const [low, high] = limits(p);
			const raw = parseNumber(p, draft);
			if (raw === undefined) {
				error = 'Неверный формат числа';
				return;
			}
			if (raw < low || raw > high) {
				error = `Допустимый диапазон: ${formatLimit(p, low)}–${formatLimit(p, high)}`;
				return;
			}
			value = raw;
		}
		if (await write(p.write ?? p.read, value)) {
			notify(p);
			closeOverlay();
		}
	}

	async function typeKey(key: string) {
		if (key === 'Escape') {
			closeOverlay();
		} else if (key === 'Enter') {
			await finishInput();
		} else if (editing) {
			switch (key) {
			case 'Back Space':
				draft = draft.slice(0, -1);
				break;
			case 'Clear':
				draft = '';
				break;
			default:
				draft = replaceOnKey ? key : draft + key;
			}
			replaceOnKey = false;
		}
	}

	async function signIn(event: SubmitEvent) {
		event.preventDefault();
		const candidate = tokenEntry.trim();
		try {
			const response = await fetch('/api/write-access', { headers: { 'X-Nautilus-Token': candidate } });
			if (!response.ok) {
				error = (await response.text()).trim();
				return;
			}
			token = candidate;
			error = '';
			closeOverlay();
		} catch {
			error = 'Нет связи с Nautilus';
		}
	}
</script>

<svelte:head><title>{doc.name}</title></svelte:head>
<main>
	<header>
		<span class="connection" class:offline={!client.connected} aria-live="polite">
			{!client.connected ? 'Нет связи с Nautilus' : Object.keys(client.frame?.quality ?? {}).length ? 'Нет свежих данных PLC' : 'Подключено'}
		</span>
		{#if fileList}<button type="button" onclick={() => openOverlay(fileList.id)}>Файлы</button>{/if}
		{#if login}
			<button type="button" onclick={() => token ? token = '' : openOverlay(login.id)}>
				{token ? 'Удалить токен' : 'Токен записи'}
			</button>
		{/if}
	</header>
	<div class="fit" bind:clientWidth={outerWidth} style:height={doc.height * scale + 'px'}>
		<div class="canvas" style:width={doc.width + 'px'} style:height={doc.height + 'px'} style:transform={'scale(' + scale + ')'}>
			{#if primary}<Screen screen={primary} {read} {editValue} {canActivate} signedIn={!!token} {files} {activate} />{/if}
			{#if common && common.id !== active}<Screen screen={{ ...common, background: 'transparent' }}
				{read} {editValue} {canActivate} signedIn={!!token} {files} {activate} />{/if}
			{#each direct as screen (screen.id)}
				<Screen {screen} {read} {editValue} {canActivate} signedIn={!!token} {files} {activate} overlay />
			{/each}
			{#if selected && selected.id !== login?.id}
				<div class="modal">
					<Screen screen={centered(selected)} {read} {editValue} {canActivate} signedIn={!!token} {files} {activate} overlay />
				</div>
			{/if}
			{#if !client.frame && doc.logo}
				<img class="splash" src={doc.logo} alt="" />
			{/if}
		</div>
	</div>
	{#if selected?.id === login?.id && selected}
		<form class="operation" onsubmit={signIn}>
			<label>Токен записи Nautilus <input type="password" bind:value={tokenEntry} required autocomplete="off" /></label>
			<button>Применить</button><button type="button" onclick={closeOverlay}>Отмена</button>
		</form>
	{:else if editing}
		<form class="operation" onsubmit={(event) => { event.preventDefault(); void finishInput(); }}>
			{#if isPassword(editing)}
				<label>Значение <input type="password" inputmode="numeric" autocomplete="off" bind:value={draft} /></label>
			{:else}
				<label>Значение <input bind:value={draft} oninput={() => replaceOnKey = false} /></label>
			{/if}
			<button>ОК</button><button type="button" onclick={closeOverlay}>Отмена</button>
		</form>
	{:else if pending}
		<div class="operation">
			<span>Подтвердить действие?</span>
			<button type="button" onclick={() => finishConfirm(true)}>Да</button>
			<button type="button" onclick={() => finishConfirm(false)}>Отмена</button>
		</div>
	{:else if selected}
		<div class="operation">
			{#if selected.id === fileList?.id}
				<label>Файлы <input type="file" multiple onchange={(event) => {
					files = Array.from(event.currentTarget.files ?? [], (file) => file.name);
					registers.LW9332 = files[0] ?? '';
				}} /></label>
			{/if}
			<button type="button" onclick={closeOverlay}>Закрыть</button>
		</div>
	{/if}
	{#if error}<p role="alert">{error}</p>{/if}
</main>

<style>
	:global(body) { margin: 0; background: #171b1f; color: #eef0f2; font: 14px system-ui, sans-serif; }
	main { max-width: 1000px; margin: auto; padding: 12px; }
	header, .operation { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 10px; }
	.connection { margin-right: auto; color: #c1d9c0; }
	.connection.offline, p { color: #ffd09c; }
	.fit { width: 100%; position: relative; }
	.canvas { position: absolute; transform-origin: top left; overflow: hidden; background: #000; }
	.modal { position: absolute; inset: 0; background: #0007; }
	.splash { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; pointer-events: none; }
	.operation { margin-top: 10px; }
	input, button { font: inherit; }
	button { padding: 4px 9px; }
</style>
