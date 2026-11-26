<script lang="ts">
	import { allows, box, displayNumber, labelStyle, type Part, type Register, type ScreenModel, type Value } from './model';

	let {
		screen, read, editValue, signedIn, canActivate, activate, files, overlay = false,
	}: {
		screen: ScreenModel;
		read: (r?: Register) => Value;
		editValue: (p: Part) => string | undefined;
		signedIn: boolean;
		canActivate: (p: Part) => boolean;
		activate: (p: Part) => void;
		files: string[];
		overlay?: boolean;
	} = $props();

	const actionable = ['Switch', 'Set Bit', 'Word Set', 'Function Key', 'Number Input', 'Text Input'];

	function state(p: Part): number {
		if (p.kind === 'Vector graph') return p.mode ?? 0;
		if (p.kind === 'Word Lamp') return Number(p.meta?.[String(read(p.read))] ?? 0);
		if (p.kind === 'Switch' || p.kind === 'Bit Lamp') return Number(Boolean(read(p.read)));
		return 0;
	}

	function shownFiles(p: Part): string[] {
		const suffixes = p.meta?.suffix?.split(';').map((value) => value.replace(/^\*/, '').toLowerCase()) ?? [];
		return suffixes.length ? files.filter((name) => suffixes.some((suffix) => name.toLowerCase().endsWith(suffix))) : files;
	}

	function text(p: Part, selected: number): string {
		if (p.kind === 'Text Input' || p.kind === 'Text Display' || p.kind === 'Number Input') {
			const value = read(p.read);
			if (p.kind !== 'Number Input') return value === 0 ? '' : String(value);
			return editValue(p) ?? displayNumber(p, value);
		}
		return (p.labels?.[selected] ?? p.labels?.[0])?.text ?? '';
	}
</script>

<div class="screen" class:overlay style={box(screen.bounds)} style:background={screen.background} aria-label={screen.name}>
	{#each screen.parts as p, index (index)}
		{#if p.drawing}
			{@const d = p.drawing}
			{@const b = d.bounds}
			{#if d.kind === 'text' && d.text}
				<div class="drawing text" style={box(b) + ';' + labelStyle(d.text)}>{d.text.text}</div>
			{:else}
				<svg class="drawing" width={screen.bounds.w} height={screen.bounds.h} aria-hidden="true">
					{#if d.kind === 'line'}
						<line x1={b.x} y1={b.y} x2={d.x2} y2={d.y2} stroke={d.stroke} stroke-width={d.strokeWidth} />
					{:else if d.kind === 'polygon'}
						<polygon points={d.points?.map((point) => point.x + ',' + point.y).join(' ')}
							fill={d.fill} stroke={d.stroke} stroke-width={d.strokeWidth} />
					{:else if d.kind === 'image'}
						<image x={b.x} y={b.y} width={b.w} height={b.h} href={d.image} preserveAspectRatio="none" />
					{:else}
						{#if d.endFill}
							<defs><linearGradient id={'gradient-' + screen.id + '-' + index} x2="0" y2="1">
								<stop offset="0" stop-color={d.fill} /><stop offset="1" stop-color={d.endFill} />
							</linearGradient></defs>
						{/if}
						<rect x={b.x} y={b.y} width={b.w} height={b.h} rx={d.radius ?? 0}
							fill={d.endFill ? 'url(#gradient-' + screen.id + '-' + index + ')' : d.fill}
							stroke={d.stroke} stroke-width={d.strokeWidth} />
					{/if}
				</svg>
			{/if}
		{:else if p.kind !== 'Direct Window' && allows(p.visible, read, signedIn)}
			{@const selected = state(p)}
			{@const src = p.assets?.[selected] ?? p.assets?.[0]}
			{@const label = p.labels?.[selected] ?? p.labels?.[0]}
			{@const content = text(p, selected)}
			{#snippet appearance()}
				{#if src}<img src={src} alt="" draggable="false" />{/if}
				{#if content}
					<span class="label" style={(p.labelBox ? box(p.labelBox) : '') + ';' + (label ? labelStyle(label) : '')}>{content}</span>
				{/if}
			{/snippet}
			{#if actionable.includes(p.kind) && p.meta?.navigation !== 'Popup window title bar'}
				<button type="button" class="part" class:plain={!src && (p.kind !== 'Function Key' || !!content.trim())} style={box(p.bounds)}
					aria-label={content.replace(/\s+/g, ' ').trim() || p.char || p.name}
					disabled={!canActivate(p)}
					onclick={() => activate(p)}>
					{@render appearance()}
				</button>
			{:else if p.kind === 'FileList'}
				<ul class="part files" style={box(p.bounds)} aria-label="Выбранные файлы">
					{#each shownFiles(p) as file}<li>{file}</li>{/each}
				</ul>
			{:else}
				<div class="part display" style={box(p.bounds)} data-part={p.name}>
					{@render appearance()}
				</div>
			{/if}
		{/if}
	{/each}
</div>

<style>
	.screen { position: absolute; overflow: hidden; pointer-events: none; }
	.overlay { box-shadow: 0 0 0 1px #8b9299, 0 8px 28px #000a; }
	.drawing { position: absolute; top: 0; left: 0; pointer-events: none; }
	.text, .label { white-space: pre-line; line-height: 1.1; }
	.text { overflow: hidden; }
	.part { position: absolute; box-sizing: border-box; margin: 0; }
	.display { pointer-events: none; }
	button.part { border: 0; padding: 0; background: transparent; cursor: pointer; font: inherit; pointer-events: auto; }
	button.plain { background: #eceff1; color: #111; border: 1px solid #6a747b; }
	button:disabled { opacity: .55; cursor: default; }
	button:focus-visible { outline: 3px solid #69b8ff; outline-offset: -3px; }
	img { display: block; width: 100%; height: 100%; object-fit: fill; pointer-events: none; }
	.label { position: absolute; inset: 0; align-content: center; overflow: hidden; pointer-events: none; }
	.files { background: #fff; color: #111; overflow: auto; padding: 4px 20px; pointer-events: auto; }
</style>
