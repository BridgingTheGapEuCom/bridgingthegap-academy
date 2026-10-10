<template>
	<dl class="btg-metadata-strip" :class="`btg-metadata-strip--${layout}`">
		<div
			v-for="item in items"
			:key="item.label"
			class="btg-metadata-strip__item"
		>
			<dt>{{ item.label }}</dt>
			<dd :title="item.value">
				<BtgIcon
					style="margin-right: 0.25rem"
					v-if="item.icon"
					:name="item.icon"
					decorative
					size="small"
				/><span class="btg-metadata-strip__value">{{
					item.value
				}}</span>
			</dd>
		</div>
	</dl>
</template>

<script setup lang="ts">
import BtgIcon from "./BtgIcon.vue";
import type { BtgIconName } from "./btg-icon-names";

withDefaults(
	defineProps<{
		items: { label: string; value: string; icon?: BtgIconName }[];
		layout?: "default" | "two-column" | "three-column";
	}>(),
	{ layout: "default" },
);
</script>
