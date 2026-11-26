export type Value = boolean | number | string;
export type Bounds = { x: number; y: number; w: number; h: number };
export type Register = { key: string; area: 'LB' | 'LW' | 'CIO'; index: number; count: number };
export type Condition = { register?: Register; on: boolean; login?: boolean };
export type Label = { text: string; size: number; color: string; align: number; bold: boolean };
export type Draw = {
	kind: string;
	bounds: Bounds;
	x2: number;
	y2: number;
	stroke: string;
	strokeWidth: number;
	fill: string;
	endFill?: string;
	radius?: number;
	points?: { x: number; y: number }[];
	text?: Label;
	image?: string;
};
export type Part = {
	kind: string;
	name: string;
	bounds: Bounds;
	mode?: number;
	interval?: number;
	function?: string;
	char?: string;
	target?: number;
	keyboard?: number;
	confirm?: boolean;
	autoLogin?: boolean;
	parameters?: number[];
	labels?: Label[];
	labelBox?: Bounds;
	assets?: Record<number, string>;
	read?: Register;
	write?: Register;
	low?: Register;
	high?: Register;
	visible?: Condition;
	enabled?: Condition;
	drawing?: Draw;
	meta?: Record<string, string>;
};
export type ScreenModel = { id: number; name: string; background: string; bounds: Bounds; parts: Part[] };
export type HMIModel = {
	version: number;
	name: string;
	title: string;
	start: number;
	common: number;
	fast: number;
	width: number;
	height: number;
	logo?: string;
	windows: ScreenModel[];
};

export function box(b: Bounds): string {
	return `left:${b.x}px;top:${b.y}px;width:${b.w}px;height:${b.h}px`;
}

export function allows(condition: Condition | undefined, read: (r?: Register) => Value, signedIn: boolean): boolean {
	if (!condition) return true;
	return (!condition.login || signedIn) && (!condition.register || Boolean(read(condition.register)) === condition.on);
}

export function labelStyle(label: Label): string {
	const align = label.align === 1 ? 'right' : label.align === 2 ? 'center' : 'left';
	return `font-size:${label.size}px;color:${label.color};text-align:${align};font-weight:${label.bold ? 700 : 400}`;
}

export function numberScale(part: Part): number {
	return 10 ** (part.parameters?.[2] ?? 0);
}

export function isPassword(part: Part): boolean {
	return part.kind === 'Number Input' && part.meta?.numberFormat === '4';
}

export function displayNumber(part: Part, raw: Value): string {
	if (isPassword(part)) return '*****';
	const value = Number(raw);
	switch (part.meta?.numberFormat) {
	case '2': return value.toString(16).toUpperCase();
	case '3': return value.toString(2);
	default: return (value / numberScale(part)).toFixed(part.parameters?.[2] ?? 0);
	}
}

// NB stores fixed-point values as integers in its word registers.
export function parseNumber(part: Part, text: string): number | undefined {
	const value = text.trim();
	const format = part.meta?.numberFormat;
	let raw: number;
	switch (format) {
	case '2':
		if (!/^[0-9a-f]+$/i.test(value)) return;
		raw = Number.parseInt(value, 16);
		break;
	case '3':
		if (!/^[01]+$/.test(value)) return;
		raw = Number.parseInt(value, 2);
		break;
	default: {
		const match = /^([+-]?)([0-9]+)(?:[.,]([0-9]+))?$/.exec(value);
		const places = part.parameters?.[2] ?? 0;
		if (!match || (match[1] === '-' && format !== '0') || (format === '4' && match[1])) return;
		const fraction = match[3] ?? '';
		if (fraction.length > places) return;
		raw = Number(match[1] + match[2] + fraction.padEnd(places, '0'));
	}
	}
	return Number.isSafeInteger(raw) ? raw : undefined;
}
