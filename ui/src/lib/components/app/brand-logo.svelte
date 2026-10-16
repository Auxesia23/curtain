<script lang="ts">
	/**
	 * Curtain logo: an open curtain with a star behind it.
	 * The simple, mysterious design takes inspiration from tarot and the occult.
	 * It uses `currentColor` to follow the active light or dark theme.
	 */
	let { class: className = 'size-5' }: { class?: string } = $props();
</script>

<svg
	viewBox="0 0 24 24"
	class={className}
	fill="none"
	xmlns="http://www.w3.org/2000/svg"
	aria-hidden="true"
>
	<!-- Curtain rod -->
	<path d="M3.2 4.2h17.6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
	<!-- Left curtain panel, pulled to the side -->
	<path
		d="M5 4.2c.9 5.4.5 9.3-1.6 13.4 3.2-1.2 6-1.7 8.6-1.7"
		stroke="currentColor"
		stroke-width="1.7"
		stroke-linecap="round"
		stroke-linejoin="round"
	/>
	<!-- Right curtain panel -->
	<path
		d="M19 4.2c-.9 5.4-.5 9.3 1.6 13.4-3.2-1.2-6-1.7-8.6-1.7"
		stroke="currentColor"
		stroke-width="1.7"
		stroke-linecap="round"
		stroke-linejoin="round"
	/>
	<!-- Star behind the curtain -->
	<path
		d="M12 5.8c.6 2.6 1.5 3.5 4.1 4.1-2.6.6-3.5 1.5-4.1 4.1-.6-2.6-1.5-3.5-4.1-4.1 2.6-.6 3.5-1.5 4.1-4.1z"
		fill="currentColor"
	/>
</svg>
