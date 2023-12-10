<script>
	let counter = 0;
</script>

<button on:click={() => (counter += 1)}>
	Clicked {counter}
	{counter === 1 ? "time" : "times"}
</button>
