<script lang="ts">
	import { omronValue, type OmronVariable, type OmronWatchMessage } from './ladder';

	let { rows, watch, error, open }: {
		rows: OmronVariable[];
		watch: OmronWatchMessage;
		error?: string;
		open: boolean;
	} = $props();

	let filter = $state('');
	const shown = $derived(rows.filter(row => `${row.scope || 'Global'} ${row.name} ${row.address} ${row.type}`
		.toLowerCase().includes(filter.trim().toLowerCase())));
	const cpu = $derived(watch.sample?.cpu);
	const mode = $derived(cpu?.mode === 0 ? 'PROGRAM' : cpu?.mode === 2 ? 'MONITOR' : cpu?.mode === 4 ? 'RUN' : 'UNKNOWN');
	const statusText = $derived(error ?? watch.error ?? (cpu?.connected ? `CPU ${mode} · sampled memory` :
		watch.active ? (cpu ? 'Connection interrupted' : 'Connecting…') : 'Offline'));
	const showState = $derived(watch.active || !!watch.error || !!error || open);
</script>

{#if showState}
	<div class="status" role="status" data-testid="omron-watch-status">
		{#if watch.endpoint}<span>{watch.endpoint} · </span>{/if}{statusText}
		{#if cpu?.lastError}<span title={cpu.lastError}> · {cpu.lastError}</span>{/if}
	</div>
{/if}
{#if open}
	<section aria-label="Omron variables">
		<label>
			<span>Filter variables</span>
			<input type="search" bind:value={filter} placeholder="Name, address, task or type" />
		</label>
		<div class="scroll">
			<table>
				<thead><tr><th scope="col">Scope</th><th scope="col">Symbol</th><th scope="col">FINS address</th><th scope="col">Type</th><th scope="col">Value</th><th scope="col">Quality</th></tr></thead>
				<tbody>
					{#each shown as row (row.scope + ':' + row.name)}
						{@const quality = row.key ? watch.sample?.quality[row.key] : undefined}
						{@const value = row.key && (quality === 'good' || quality === 'stale') ? omronValue(watch.sample?.values[row.key]) : ''}
						<tr data-scope={row.scope} data-symbol={row.name}>
							<td>{row.scope || 'Global'}</td>
							<th scope="row">{row.name}</th>
							<td>{row.address || '—'}</td>
							<td>{row.type}</td>
							<td>{value || '—'}</td>
							<td class:good={quality === 'good'}>{row.key ? quality ?? (watch.active ? 'waiting' : 'offline') : 'unavailable'}</td>
						</tr>
					{/each}
					{#if !shown.length}<tr><td colspan="6">No matching variables</td></tr>{/if}
				</tbody>
			</table>
		</div>
	</section>
{/if}

<style>
	.status { padding: 5px 10px; color: var(--vscode-descriptionForeground); overflow-wrap: anywhere; }
	section { border-block: 1px solid var(--vscode-panel-border); background: var(--vscode-editorWidget-background); }
	label { display: flex; gap: 8px; align-items: center; padding: 6px 10px; }
	input {
		background: var(--vscode-input-background); color: var(--vscode-input-foreground);
		border: 1px solid var(--vscode-input-border, var(--vscode-panel-border)); padding: 4px 6px;
		width: min(260px, 100%);
	}
	.scroll { overflow: auto; max-height: min(33vh, 260px); }
	table { width: 100%; border-collapse: collapse; font-family: var(--vscode-editor-font-family, monospace); font-size: 12px; }
	th, td { text-align: left; padding: 4px 10px; border-bottom: 1px solid var(--vscode-panel-border); white-space: nowrap; }
	thead { position: sticky; top: 0; background: var(--vscode-editorWidget-background); }
	tbody th { font-weight: normal; }
	.good { color: var(--vscode-testing-iconPassed); }
</style>
