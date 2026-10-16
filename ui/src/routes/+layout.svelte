<script lang="ts">
	import './layout.css';
	import { ModeWatcher } from 'mode-watcher';
	import { Toaster } from '#lib/components/ui/sonner';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<svelte:head>
	<title>Curtain</title>
	<meta name="description" content="Zero-knowledge environment variable manager" />
</svelte:head>

<ModeWatcher />
{@render children()}
<Toaster position="top-right" richColors closeButton />
