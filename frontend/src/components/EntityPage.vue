
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import type { DomainRecord, EntityConfig, PriorityDecisionRevision } from '../types/domain';
import { allowedTargets, formatDate } from '../utils/format';
import { useAuth } from '../hooks/useAuth';
import StatusBadge from './common/StatusBadge.vue';
import SeverityBadge from './common/SeverityBadge.vue';
import EvidenceGallery from './common/EvidenceGallery.vue';
import MetricCard from './common/MetricCard.vue';
import ConfirmDialog from './common/ConfirmDialog.vue';

const props = withDefaults(defineProps<{ config: EntityConfig; store: any; showEvidence?: boolean }>(), { showEvidence: false });
const { session, canAtLeast } = useAuth();
const search = ref('');
const showCreate = ref(false);
const pending = ref<{ item: DomainRecord; status: string } | null>(null);
const releasing = ref<DomainRecord | null>(null);
const canWrite = computed(() => canAtLeast('operator'));
const highRisk = computed(() => props.store.items.filter((item: DomainRecord) => ['high', 'critical'].includes(item.riskLevel)).length);
const releasableCount = computed(() => props.store.items.filter((item: DomainRecord) => item.releaseEligible).length);

onMounted(() => void props.store.load(props.config.path));

function targetsFor(item: DomainRecord): readonly string[] {
	if (props.config.key === 'priorityDecision') {
		if (!canAtLeast('reviewer') || item.preparedBy === session.value?.username) return [];
	} else if (!canWrite.value) return [];
	return allowedTargets(props.config.key, item.status);
}

// Only a reviewer/admin who is not the preparer may close out a finalized
// restrict/urgent decision, and only when the backend marks it releasable.
function canRelease(item: DomainRecord): boolean {
	return props.config.key === 'priorityDecision'
		&& canAtLeast('reviewer')
		&& item.preparedBy !== session.value?.username
		&& item.releaseEligible === true;
}

function isActiveRequirement(item: DomainRecord): boolean {
	return item.status === 'restrict' || item.status === 'urgent';
}

function latestRevision(item: DomainRecord): PriorityDecisionRevision | undefined {
	return item.revisions?.[item.revisions.length - 1];
}

async function createDemo() {
	const now = Date.now();
	const created = await props.store.createRecord(props.config.path, {
		code: `${props.config.key.toUpperCase()}-${String(now).slice(-6)}`, name: `新增${props.config.label}`,
		description: '通过前端工作台创建的业务记录', facility: 'K42 桥梁作业区', owner: session.value?.displayName || '现场操作员',
		category: '结构复核', riskLevel: 'medium', metricValue: 25, metricUnit: 'score', effectiveAt: new Date().toISOString(),
		evidence: '现场照片、量测记录与检查批次已完成核对', relatedCode: props.config.key === 'priorityDecision' ? 'DF-001' : 'IR-001',
	});
	if (created) { search.value = ''; showCreate.value = false; }
}

async function confirmTransition() {
	if (!pending.value) return;
	const changed = await props.store.transition(props.config.path, pending.value.item, pending.value.status);
	if (changed) { search.value = ''; pending.value = null; }
}

async function confirmRelease() {
	if (!releasing.value) return;
	const item = releasing.value;
	const changed = await props.store.release(props.config.path, item, '同桥缺陷均已缓解或关闭，复核人解除现行要求');
	if (changed) releasing.value = null;
}
</script>

<template>
	<main class="workspace">
		<header class="page-header">
			<div><p class="eyebrow">业务工作台</p><h1>{{ config.label }}</h1><p>统一管理{{ config.label }}的状态、风险、证据与责任人。</p></div>
			<el-button v-if="canWrite" type="primary" @click="showCreate = true">新增{{ config.label }}</el-button>
		</header>
		<section class="metrics">
			<MetricCard label="记录总数" :value="store.meta.total" detail="当前筛选范围"/>
			<MetricCard label="高风险" :value="highRisk" detail="需要优先复核"/>
			<MetricCard v-if="config.key === 'priorityDecision'" label="可解除" :value="releasableCount" detail="同桥缺陷已全部缓解或关闭"/>
			<MetricCard :label="config.key === 'priorityDecision' ? '待复核草稿' : '状态种类'" :value="config.key === 'priorityDecision' ? store.items.filter((item: DomainRecord) => item.status === 'draft').length : new Set(store.items.map((item: DomainRecord) => item.status)).size" :detail="config.key === 'priorityDecision' ? '等待独立复核定稿' : '状态机覆盖'"/>
		</section>
		<section v-if="showEvidence" class="evidence-panel"><header><strong>证据摘要</strong><span>最近四条记录</span></header><EvidenceGallery :records="store.items"/></section>
		<section class="toolbar"><el-input v-model="search" :placeholder="`搜索${config.label}编码或名称`" clearable/><el-button type="primary" @click="store.load(config.path, search)">查询</el-button><el-button @click="search = ''; store.load(config.path)">重置</el-button></section>
		<el-alert v-if="store.error" :title="store.error" type="error" show-icon/>
		<section class="table-shell">
			<el-table v-loading="store.loading" :data="store.items">
				<el-table-column prop="code" label="编码" width="150"/>
				<el-table-column label="名称" min-width="180"><template #default="{ row }"><strong>{{ row.name }}</strong><small>{{ row.facility }}</small></template></el-table-column>
				<el-table-column label="状态" width="130"><template #default="{ row }"><StatusBadge :status="row.status"/></template></el-table-column>
				<el-table-column label="风险" width="90"><template #default="{ row }"><SeverityBadge v-if="['defectFinding', 'priorityDecision'].includes(config.key)" :severity="row.riskLevel"/><span v-else>{{ row.riskLevel }}</span></template></el-table-column>
				<el-table-column prop="owner" label="责任人" min-width="130"/>
				<el-table-column label="指标" width="120"><template #default="{ row }">{{ row.metricValue }} {{ row.metricUnit }}</template></el-table-column>
				<el-table-column v-if="config.key === 'priorityDecision'" label="版本审计" width="250"><template #default="{ row }"><strong>v{{ row.version }} · {{ row.preparedBy }}</strong><small>{{ latestRevision(row)?.actor }} · {{ latestRevision(row)?.requestId }}</small><small>{{ latestRevision(row)?.evidence }}</small></template></el-table-column>
				<el-table-column v-if="config.key === 'priorityDecision'" label="解除收尾" min-width="240"><template #default="{ row }">
					<div v-if="row.status === 'released'" class="release-cell"><span class="release-tag release-tag--done">已解除</span><small>不再是现行要求</small></div>
					<div v-else-if="isActiveRequirement(row) && row.releaseEligible" class="release-cell"><span class="release-tag release-tag--ready">可以解除</span><small>同桥缺陷均已缓解或关闭</small></div>
					<div v-else-if="isActiveRequirement(row)" class="release-cell"><span class="release-tag release-tag--pending">暂不能解除</span><small>未处理完缺陷：{{ (row.outstandingDefectCodes && row.outstandingDefectCodes.length) ? row.outstandingDefectCodes.join('、') : '无' }}</small></div>
					<span v-else class="muted">未定稿，无需收尾</span>
				</template></el-table-column>
				<el-table-column label="更新时间" width="180"><template #default="{ row }">{{ formatDate(row.updatedAt) }}</template></el-table-column>
				<el-table-column label="操作" width="300"><template #default="{ row }"><div class="row-actions"><el-button v-for="target in targetsFor(row)" :key="target" link type="primary" @click="pending = { item: row, status: target }">推进至 {{ target }}</el-button><el-button v-if="canRelease(row)" link type="success" @click="releasing = row">解除</el-button><span v-if="targetsFor(row).length === 0 && !canRelease(row)" class="muted">无可用操作</span></div></template></el-table-column>
			</el-table>
		</section>
		<ConfirmDialog v-model="showCreate" :title="`新增${config.label}`" @confirm="createDemo"><p>将创建一条包含完整责任人、风险和证据信息的记录。</p></ConfirmDialog>
		<ConfirmDialog :model-value="Boolean(pending)" title="确认状态迁移" @update:model-value="pending = null" @confirm="confirmTransition"><p>状态迁移会写入审计日志并保留请求号；优先级定稿后不可覆盖。</p><strong>{{ pending?.item.status }} → {{ pending?.status }}</strong></ConfirmDialog>
		<ConfirmDialog :model-value="Boolean(releasing)" title="确认解除现行要求" @update:model-value="releasing = null" @confirm="confirmRelease"><p>解除由复核人执行，拟制人不能解除自己的决定。解除后该限速/立即处置记为「已解除」，不再是现行要求。</p><strong>{{ releasing?.code }} · {{ releasing?.status }} → released</strong></ConfirmDialog>
	</main>
</template>
