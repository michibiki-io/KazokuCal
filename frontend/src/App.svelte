<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { Button, Checkbox, Input, Modal } from 'flowbite-svelte';
  import {
    Check,
    Copy,
    Download,
    Eye,
    Github,
    Globe,
    GripVertical,
    Plus,
    Replace,
    Trash2,
    Upload,
    UserRound,
    UsersRound
  } from 'lucide-svelte';

  type WeekStartsOn = 'monday' | 'sunday';
  type ItemColor = 'black' | 'red' | 'blue';
  type ScheduleScope = 'personal' | 'group' | 'world';
  type TeleworkStatus = { papa: boolean; mama: boolean };
  type UserInfo = {
    authenticated: boolean;
    user?: string;
    email?: string;
    groups: string[];
  };
  type ScheduleItem = {
    id: string;
    sourceId?: string;
    date: string;
    text: string;
    color: ItemColor;
    scopeType?: ScheduleScope;
    group?: string;
  };
  type MultiDayScheduleItem = {
    id: string;
    sourceId?: string;
    startDate: string;
    endDate: string;
    text: string;
    color: ItemColor;
    arrow: boolean;
    scopeType?: ScheduleScope;
    group?: string;
  };
  type CalendarData = {
    year: number;
    month: number;
    weekStartsOn: WeekStartsOn;
    telework: Record<string, TeleworkStatus>;
    scheduleItems: ScheduleItem[];
    multiDayItems: MultiDayScheduleItem[];
  };
  type Segment = {
    id: string;
    row: number;
    startCol: number;
    endCol: number;
    lane: number;
    text: string;
    color: ItemColor;
    arrow: boolean;
    isLast: boolean;
  };

  declare const __APP_VERSION__: string;
  declare const __GITHUB_REPOSITORY__: string;
  declare const __RELEASE_COMMIT__: string;

  const legacyStorageKey = 'kazokucal.calendar.v1';
  const appVersion = formatAppVersion(__APP_VERSION__);
  const releaseCommit = __RELEASE_COMMIT__;
  const shortReleaseCommit = releaseCommit ? releaseCommit.slice(0, 7) : '';
  const githubRepositoryUrl = __GITHUB_REPOSITORY__ ? `https://github.com/${__GITHUB_REPOSITORY__}` : '';
  const releaseCommitUrl = githubRepositoryUrl && releaseCommit ? `${githubRepositoryUrl}/commit/${releaseCommit}` : githubRepositoryUrl;
  const monthNames = [
    '',
    'January',
    'February',
    'March',
    'April',
    'May',
    'June',
    'July',
    'August',
    'September',
    'October',
    'November',
    'December'
  ];
  const weekdays = {
    0: { jp: '日', en: 'Sun' },
    1: { jp: '月', en: 'Mon' },
    2: { jp: '火', en: 'Tue' },
    3: { jp: '水', en: 'Wed' },
    4: { jp: '木', en: 'Thu' },
    5: { jp: '金', en: 'Fri' },
    6: { jp: '土', en: 'Sat' }
  } as const;
  const colorOptions: ItemColor[] = ['black', 'red', 'blue'];
  const colorLabels: Record<ItemColor, string> = { black: '黒', red: '赤', blue: '青' };

  let data: CalendarData = defaultData();
  let userInfo: UserInfo = { authenticated: false, groups: [] };
  let userMenuOpen = false;
  let showPersonalSchedules = true;
  let showGroupSchedules = true;
  let showWorldSchedules = true;
  let ready = false;
  let loadBusy = false;
  let saveTimer: ReturnType<typeof setTimeout> | undefined;
  let holidays: Record<string, string> = {};
  let loadedHolidayYear: number | null = null;
  let selectedDayKey = '';
  let dayModalOpen = false;
  let dayEditorMode: 'schedule' | 'multi' = 'schedule';
  let multiDayModalOpen = false;
  let selectedMultiDayId = '';
  let newScheduleText = '';
  let newScheduleColor: ItemColor = 'black';
  let newScheduleScopeType: ScheduleScope = 'personal';
  let newScheduleGroup = '';
  let newMultiDay: MultiDayScheduleItem = blankMultiDay();
  let newMultiDayScopeType: ScheduleScope = 'personal';
  let newMultiDayGroup = '';
  let dayModalMessage = '';
  let replaceOnImport = false;
  let importBusy = false;
  let importFileInput: HTMLInputElement | null = null;
  let importResultOpen = false;
  let importResultTitle = '';
  let importResultMessage = '';
  let statusMessage = '';
  let downloadBusy = false;
  let previewBusy = false;
  let draggingScheduleId = '';
  let dragOverScheduleId = '';
  let copyFeedbackKey = '';
  let copyFeedbackTimer: ReturnType<typeof setTimeout> | undefined;

  $: availableGroups = userInfo.groups ?? [];
  $: userDisplayName = userInfo.user ?? userInfo.email ?? 'authenticated';
  $: supportsGroupSchedules = availableGroups.length > 0;
  $: canEditWorldSchedules = true;
  $: canChoosePersonalSchedules = userInfo.authenticated;
  $: showScopeSelector = canChoosePersonalSchedules || supportsGroupSchedules;
  $: scheduleVisibilityKey = `${showPersonalSchedules}-${showGroupSchedules}-${showWorldSchedules}`;
  $: scopeVisibility = {
    personal: showPersonalSchedules,
    group: showGroupSchedules,
    world: showWorldSchedules
  };
  $: grid = buildGrid(data.year, data.month, data.weekStartsOn);
  $: weekdayOrder = data.weekStartsOn === 'sunday' ? [0, 1, 2, 3, 4, 5, 6] : [1, 2, 3, 4, 5, 6, 0];
  $: selectedTelework = data.telework[selectedDayKey] ?? { papa: false, mama: false };
  $: visibleScheduleItems = scheduleVisibilityKey
    ? data.scheduleItems.filter((item) => isScheduleItemVisible(item))
    : data.scheduleItems.filter((item) => isScheduleItemVisible(item));
  $: visibleScheduleItemsByDate = groupScheduleItemsByDate(visibleScheduleItems);
  $: visibleMultiDayItems = scheduleVisibilityKey
    ? data.multiDayItems.filter((item) => isMultiDayItemVisible(item))
    : data.multiDayItems.filter((item) => isMultiDayItemVisible(item));
  $: selectedItems = visibleScheduleItems.filter((item) => item.date === selectedDayKey);
  $: selectedMultiDayItem = visibleMultiDayItems.find((item) => item.id === selectedMultiDayId);
  $: multiDaySegments = buildMultiDaySegments(visibleMultiDayItems, grid);
  $: multiDaySegmentsByRow = groupSegmentsByRow(multiDaySegments, grid.length);
  $: pdfBusy = downloadBusy || previewBusy;
  $: if (ready && data.year !== loadedHolidayYear) {
    void loadHolidays(data.year);
  }

  onMount(() => {
    localStorage.removeItem(legacyStorageKey);
    void initializeApp();
  });

  onDestroy(() => {
    removeScheduleDragListeners();
    clearCopyFeedbackTimer();
  });

  function apiPath(path: string): string {
    return `api/${path.replace(/^\/+/, '')}`;
  }

  function handleUserMenuFocusOut(event: FocusEvent) {
    const nextTarget = event.relatedTarget;
    if (nextTarget instanceof Node && event.currentTarget instanceof HTMLElement && event.currentTarget.contains(nextTarget)) return;
    userMenuOpen = false;
  }

  function formatAppVersion(version: string): string {
    const normalized = version.trim();
    if (!normalized) return '';
    if (normalized.startsWith('v') || !/^\d+\.\d+\.\d+/.test(normalized)) return normalized;
    return `v${normalized}`;
  }

  async function initializeApp() {
    const initial = defaultData();
    await loadMe();
    await loadCalendar(initial.year, initial.month, true);
  }

  async function loadMe() {
    try {
      const response = await fetch(apiPath('me'));
      if (!response.ok) throw new Error(await response.text());
      const payload = (await response.json()) as Partial<UserInfo>;
      userInfo = {
        authenticated: payload.authenticated === true,
        user: payload.user,
        email: payload.email,
        groups: Array.isArray(payload.groups)
          ? Array.from(new Set(payload.groups.map((group) => String(group).trim()).filter(Boolean)))
          : []
      };
    } catch {
      userInfo = { authenticated: false, groups: [] };
    }
    resetNewItemScopeDefaults();
  }

  function defaultData(): CalendarData {
    const now = new Date();
    return {
      year: now.getFullYear(),
      month: now.getMonth() + 1,
      weekStartsOn: 'monday',
      telework: {},
      scheduleItems: [],
      multiDayItems: []
    };
  }

  function normalizeData(value: Partial<CalendarData>): CalendarData {
    const current = defaultData();
    const year = Number(value.year ?? current.year);
    const month = Number(value.month ?? current.month);
    return {
      year: Math.min(2100, Math.max(1900, Number.isFinite(year) ? year : current.year)),
      month: Math.min(12, Math.max(1, Number.isFinite(month) ? month : current.month)),
      weekStartsOn: value.weekStartsOn === 'sunday' ? 'sunday' : 'monday',
      telework: value.telework ?? {},
      scheduleItems: Array.isArray(value.scheduleItems) ? value.scheduleItems.map(normalizeScheduleItem) : [],
      multiDayItems: Array.isArray(value.multiDayItems) ? value.multiDayItems.map(normalizeMultiDayItem) : []
    };
  }

  function normalizeScheduleItem(value: Partial<ScheduleItem>): ScheduleItem {
    const scopeType = normalizeScopeType(value.scopeType, value.group);
    return {
      id: String(value.id ?? createId()),
      sourceId: sanitizeOptionalString(value.sourceId),
      date: normalizeDateKey(value.date ?? '') || selectedDayKey || `${data.year}-${pad(data.month)}-01`,
      text: String(value.text ?? ''),
      color: coerceColor(String(value.color ?? 'black')),
      scopeType,
      group: scopeType === 'group' ? resolveGroupName(value.group) : ''
    };
  }

  function normalizeMultiDayItem(value: Partial<MultiDayScheduleItem>): MultiDayScheduleItem {
    const startDate = normalizeDateKey(value.startDate ?? '') || `${data.year}-${pad(data.month)}-01`;
    const endDate = normalizeDateKey(value.endDate ?? '') || startDate;
    const scopeType = normalizeScopeType(value.scopeType, value.group);
    return {
      id: String(value.id ?? createId()),
      sourceId: sanitizeOptionalString(value.sourceId),
      startDate,
      endDate: endDate < startDate ? startDate : endDate,
      text: String(value.text ?? ''),
      color: coerceColor(String(value.color ?? 'black')),
      arrow: value.arrow !== false,
      scopeType,
      group: scopeType === 'group' ? resolveGroupName(value.group) : ''
    };
  }

  function normalizeScopeType(value: unknown, group: unknown): ScheduleScope {
    if (value === 'group' && supportsGroupSchedules) {
      return resolveGroupName(group) ? 'group' : defaultEditableScope();
    }
    if (value === 'world') return 'world';
    if (value === 'personal') return 'personal';
    if (supportsGroupSchedules && resolveGroupName(group)) return 'group';
    return defaultEditableScope();
  }

  function defaultEditableScope(): ScheduleScope {
    return userInfo.authenticated ? 'personal' : 'world';
  }

  function scopeToggleOptions(): Array<{ value: ScheduleScope; label: string }> {
    const options: Array<{ value: ScheduleScope; label: string }> = [];
    if (canChoosePersonalSchedules) {
      options.push({ value: 'personal', label: '個人予定' });
    }
    if (supportsGroupSchedules) {
      options.push({ value: 'group', label: 'グループ予定' });
    }
    if (canEditWorldSchedules) {
      options.push({ value: 'world', label: '共通予定' });
    }
    return options;
  }

  function scopeOptionLabel(scopeType: ScheduleScope): string {
    if (scopeType === 'group') return 'グループ予定';
    if (scopeType === 'world') return '共通予定';
    return '個人予定';
  }

  function isScopeVisible(scopeType: ScheduleScope): boolean {
    if (scopeType === 'group') return showGroupSchedules;
    if (scopeType === 'world') return showWorldSchedules;
    return showPersonalSchedules;
  }

  function toggleScopeVisibility(scopeType: ScheduleScope) {
    if (scopeType === 'group') {
      showGroupSchedules = !showGroupSchedules;
      return;
    }
    if (scopeType === 'world') {
      showWorldSchedules = !showWorldSchedules;
      return;
    }
    showPersonalSchedules = !showPersonalSchedules;
  }

  function sanitizeOptionalString(value: unknown): string | undefined {
    const normalized = String(value ?? '').trim();
    return normalized ? normalized : undefined;
  }

  function resolveGroupName(value: unknown): string {
    const group = String(value ?? '').trim();
    if (!group) return availableGroups[0] ?? '';
    if (availableGroups.length === 0) return '';
    return availableGroups.includes(group) ? group : availableGroups[0] ?? '';
  }

  function resetNewItemScopeDefaults() {
    newScheduleScopeType = defaultEditableScope();
    newScheduleGroup = resolveGroupName(newScheduleGroup);
    newMultiDayScopeType = defaultEditableScope();
    newMultiDayGroup = resolveGroupName(newMultiDayGroup);
  }

  function mergeById<T extends { id: string }>(base: T[], incoming: T[]): T[] {
    const merged = [...base];
    const indexes = new Map(merged.map((item, index) => [item.id, index]));
    for (const item of incoming) {
      const index = indexes.get(item.id);
      if (index === undefined) {
        indexes.set(item.id, merged.length);
        merged.push(item);
      } else {
        merged[index] = item;
      }
    }
    return merged;
  }

  function mergeCalendarData(base: CalendarData, incoming: CalendarData): CalendarData {
    return {
      ...base,
      year: incoming.year,
      month: incoming.month,
      weekStartsOn: incoming.weekStartsOn,
      telework: { ...base.telework, ...incoming.telework },
      scheduleItems: mergeById(base.scheduleItems, incoming.scheduleItems),
      multiDayItems: mergeById(base.multiDayItems, incoming.multiDayItems)
    };
  }

  async function fetchCalendar(year: number, month: number): Promise<CalendarData> {
    const response = await fetch(apiPath(`calendar?year=${year}&month=${month}`));
    if (!response.ok) throw new Error(await response.text());
    return normalizeData((await response.json()) as Partial<CalendarData>);
  }

  function blankMultiDay(): MultiDayScheduleItem {
    const start = `${data.year}-${pad(data.month)}-01`;
    return { id: createId(), startDate: start, endDate: start, text: '', color: 'black', arrow: true };
  }

  function blankMultiDayForDate(date: string): MultiDayScheduleItem {
    const start = normalizeDateKey(date) || `${data.year}-${pad(data.month)}-01`;
    return { id: createId(), startDate: start, endDate: start, text: '', color: 'black', arrow: true };
  }

  function commitData(next: CalendarData) {
    data = next;
    scheduleSave();
  }

  function createId(): string {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
    if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
      const bytes = new Uint8Array(16);
      crypto.getRandomValues(bytes);
      bytes[6] = (bytes[6] & 0x0f) | 0x40;
      bytes[8] = (bytes[8] & 0x3f) | 0x80;
      const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
      return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
    }
    return `id-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
  }

  function pad(value: number): string {
    return String(value).padStart(2, '0');
  }

  function dateKey(day: Date): string {
    return `${day.getFullYear()}-${pad(day.getMonth() + 1)}-${pad(day.getDate())}`;
  }

  function normalizeDateKey(value: string): string {
    const match = String(value).trim().match(/^(\d{4})[-/](\d{1,2})[-/](\d{1,2})$/);
    if (!match) return '';
    return `${match[1]}-${pad(Number(match[2]))}-${pad(Number(match[3]))}`;
  }

  function parseKey(key: string): Date {
    const [year, month, day] = normalizeDateKey(key).split('-').map(Number);
    return new Date(year, month - 1, day);
  }

  function addDays(day: Date, amount: number): Date {
    return new Date(day.getFullYear(), day.getMonth(), day.getDate() + amount);
  }

  function buildGrid(year: number, month: number, weekStartsOn: WeekStartsOn): Date[][] {
    const first = new Date(year, month - 1, 1);
    const last = new Date(year, month, 0);
    const startDay = weekStartsOn === 'sunday' ? 0 : 1;
    const offset = (first.getDay() - startDay + 7) % 7;
    const start = addDays(first, -offset);
    const dayCount = Math.round((last.getTime() - start.getTime()) / 86400000) + 1;
    const weeks = Math.ceil(dayCount / 7);
    return Array.from({ length: weeks }, (_, row) =>
      Array.from({ length: 7 }, (_unused, col) => addDays(start, row * 7 + col))
    );
  }

  async function loadHolidays(year: number) {
    loadedHolidayYear = year;
    try {
      const response = await fetch(apiPath(`holidays?year=${year}`));
      if (!response.ok) throw new Error(await response.text());
      const payload = (await response.json()) as { holidays: Array<{ date: string; name: string }> };
      holidays = Object.fromEntries(payload.holidays.map((holiday) => [holiday.date, holiday.name]));
    } catch {
      holidays = {};
    }
  }

  function updateYear(value: string) {
    const year = Math.min(2100, Math.max(1900, Number(value)));
    void changeCalendarPeriod(year, data.month);
  }

  function updateMonth(value: string) {
    const month = Math.min(12, Math.max(1, Number(value)));
    void changeCalendarPeriod(data.year, month);
  }

  function updateWeekStartsOn(value: string) {
    commitData({ ...data, weekStartsOn: value === 'sunday' ? 'sunday' : 'monday' });
  }

  function openDay(day: Date) {
    selectedDayKey = dateKey(day);
    dayEditorMode = 'schedule';
    newScheduleText = '';
    newScheduleColor = 'black';
    newScheduleScopeType = defaultEditableScope();
    newScheduleGroup = availableGroups[0] ?? '';
    newMultiDay = blankMultiDayForDate(selectedDayKey);
    newMultiDayScopeType = defaultEditableScope();
    newMultiDayGroup = availableGroups[0] ?? '';
    dayModalMessage = '';
    dayModalOpen = true;
  }

  async function copyInputValue(value: string, key: string) {
    if (!value) return;
    statusMessage = '';
    try {
      await writeClipboardText(value);
      setCopyFeedback(key);
    } catch {
      statusMessage = 'クリップボードへのコピーに失敗しました。';
    }
  }

  async function writeClipboardText(value: string) {
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return;
    }
    if (typeof document === 'undefined') {
      throw new Error('Clipboard API unavailable');
    }
    const textarea = document.createElement('textarea');
    textarea.value = value;
    textarea.setAttribute('readonly', 'true');
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    const copied = document.execCommand('copy');
    document.body.removeChild(textarea);
    if (!copied) {
      throw new Error('Clipboard copy failed');
    }
  }

  function setCopyFeedback(key: string) {
    clearCopyFeedbackTimer();
    copyFeedbackKey = key;
    copyFeedbackTimer = setTimeout(() => {
      copyFeedbackKey = '';
      copyFeedbackTimer = undefined;
    }, 1600);
  }

  function clearCopyFeedbackTimer() {
    if (!copyFeedbackTimer) return;
    clearTimeout(copyFeedbackTimer);
    copyFeedbackTimer = undefined;
  }

  function switchDayEditorMode(mode: 'schedule' | 'multi') {
    dayEditorMode = mode;
    dayModalMessage = '';
    if (mode === 'multi') {
      newMultiDay = { ...newMultiDay, startDate: selectedDayKey, endDate: selectedDayKey };
    }
  }

  function handleDayKeydown(event: KeyboardEvent, day: Date) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      openDay(day);
    }
  }

  function setTelework(person: keyof TeleworkStatus, checked: boolean) {
    const current = data.telework[selectedDayKey] ?? { papa: false, mama: false };
    commitData({
      ...data,
      telework: {
        ...data.telework,
        [selectedDayKey]: { ...current, [person]: checked }
      }
    });
  }

  function addSchedule() {
    const text = newScheduleText.trim();
    if (!text) return;
    const scopeType = normalizeEditableScope(newScheduleScopeType);
    commitData({
      ...data,
      scheduleItems: [
        ...data.scheduleItems,
        {
          id: createId(),
          date: selectedDayKey,
          text,
          color: newScheduleColor,
          scopeType,
          group: scopeType === 'group' ? resolveGroupName(newScheduleGroup) : ''
        }
      ]
    });
    newScheduleText = '';
  }

  function normalizeEditableScope(scopeType: ScheduleScope): ScheduleScope {
    if (scopeType === 'group') {
      return supportsGroupSchedules && resolveGroupName(newScheduleGroup) ? 'group' : defaultEditableScope();
    }
    if (scopeType === 'world') return 'world';
    return scopeType === 'personal' ? 'personal' : defaultEditableScope();
  }

  function updateSchedule(id: string, patch: Partial<ScheduleItem>) {
    commitData({
      ...data,
      scheduleItems: data.scheduleItems.map((item) => {
        if (item.id !== id) return item;
        const merged = normalizeScheduleItem({ ...item, ...patch });
        return merged;
      })
    });
  }

  function deleteSchedule(id: string) {
    commitData({ ...data, scheduleItems: data.scheduleItems.filter((item) => item.id !== id) });
  }

  function reorderSelectedSchedule(sourceId: string, targetId: string) {
    if (sourceId === targetId) return;
    const dayItems = data.scheduleItems.filter((item) => item.date === selectedDayKey && isScheduleItemVisible(item));
    const sourceIndex = dayItems.findIndex((item) => item.id === sourceId);
    const targetIndex = dayItems.findIndex((item) => item.id === targetId);
    if (sourceIndex < 0 || targetIndex < 0) return;

    const reordered = [...dayItems];
    const [moved] = reordered.splice(sourceIndex, 1);
    reordered.splice(targetIndex, 0, moved);
    let nextDayItemIndex = 0;
    commitData({
      ...data,
      scheduleItems: data.scheduleItems.map((item) => {
        if (item.date !== selectedDayKey || !isScheduleItemVisible(item)) return item;
        return reordered[nextDayItemIndex++];
      })
    });
  }

  function startScheduleDrag(event: PointerEvent, id: string) {
    const item = selectedItems.find((candidate) => candidate.id === id);
    if (!item || isReadOnlyItem(item) || selectedItems.filter((candidate) => !isReadOnlyItem(candidate)).length < 2) return;
    draggingScheduleId = id;
    dragOverScheduleId = id;
    window.addEventListener('pointermove', handleSchedulePointerMove, { passive: false });
    window.addEventListener('pointerup', finishScheduleDrag);
    window.addEventListener('pointercancel', finishScheduleDrag);
    event.preventDefault();
  }

  function handleSchedulePointerMove(event: PointerEvent) {
    if (!draggingScheduleId) return;
    event.preventDefault();
    const target = document.elementFromPoint(event.clientX, event.clientY);
    const row = target?.closest<HTMLElement>('[data-schedule-id]');
    const targetId = row?.dataset.scheduleId;
    if (!targetId || targetId === dragOverScheduleId || !selectedItems.some((item) => item.id === targetId)) return;
    dragOverScheduleId = targetId;
    reorderSelectedSchedule(draggingScheduleId, targetId);
  }

  function finishScheduleDrag() {
    removeScheduleDragListeners();
    draggingScheduleId = '';
    dragOverScheduleId = '';
  }

  function removeScheduleDragListeners() {
    window.removeEventListener('pointermove', handleSchedulePointerMove);
    window.removeEventListener('pointerup', finishScheduleDrag);
    window.removeEventListener('pointercancel', finishScheduleDrag);
  }

  function handleScheduleHandleKeydown(event: KeyboardEvent, id: string) {
    const current = selectedItems.find((item) => item.id === id);
    if (!current || isReadOnlyItem(current)) return;
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;
    const editableItems = selectedItems.filter((item) => !isReadOnlyItem(item));
    const index = editableItems.findIndex((item) => item.id === id);
    const targetIndex = event.key === 'ArrowUp' ? index - 1 : index + 1;
    const target = editableItems[targetIndex];
    if (!target) return;
    event.preventDefault();
    reorderSelectedSchedule(id, target.id);
  }

  function addMultiDay() {
    const startDate = normalizeDateKey(newMultiDay.startDate);
    const endDate = normalizeDateKey(newMultiDay.endDate);
    if (!startDate || !endDate || startDate > endDate) {
      dayModalMessage = '期間予定の開始日と終了日を確認してください。';
      return;
    }
    const scopeType = normalizeMultiDayScope(newMultiDayScopeType);
    commitData({
      ...data,
      multiDayItems: [
        ...data.multiDayItems,
        {
          ...newMultiDay,
          id: createId(),
          startDate,
          endDate,
          scopeType,
          group: scopeType === 'group' ? resolveGroupName(newMultiDayGroup) : ''
        }
      ]
    });
    newMultiDay = blankMultiDayForDate(selectedDayKey);
    newMultiDayScopeType = defaultEditableScope();
    newMultiDayGroup = availableGroups[0] ?? '';
    dayModalMessage = '';
    dayEditorMode = 'schedule';
  }

  function normalizeMultiDayScope(scopeType: ScheduleScope): ScheduleScope {
    if (scopeType === 'group') {
      return supportsGroupSchedules && resolveGroupName(newMultiDayGroup) ? 'group' : defaultEditableScope();
    }
    if (scopeType === 'world') return 'world';
    return scopeType === 'personal' ? 'personal' : defaultEditableScope();
  }

  function updateMultiDay(id: string, patch: Partial<MultiDayScheduleItem>) {
    if (patch.startDate) patch.startDate = normalizeDateKey(patch.startDate) || patch.startDate;
    if (patch.endDate) patch.endDate = normalizeDateKey(patch.endDate) || patch.endDate;
    commitData({
      ...data,
      multiDayItems: data.multiDayItems.map((item) => {
        if (item.id !== id) return item;
        const updated = normalizeMultiDayItem({ ...item, ...patch });
        const start = normalizeDateKey(updated.startDate);
        const end = normalizeDateKey(updated.endDate);
        if (start && end && start > end) {
          if (patch.startDate) updated.endDate = start;
          if (patch.endDate) updated.startDate = end;
        }
        return updated;
      })
    });
  }

  function deleteMultiDay(id: string) {
    commitData({ ...data, multiDayItems: data.multiDayItems.filter((item) => item.id !== id) });
    if (selectedMultiDayId === id) {
      selectedMultiDayId = '';
      multiDayModalOpen = false;
    }
  }

  function openMultiDay(id: string) {
    selectedMultiDayId = id;
    multiDayModalOpen = true;
  }

  function scheduleForDate(key: string): ScheduleItem[] {
    return visibleScheduleItemsByDate[key] ?? [];
  }

  function groupScheduleItemsByDate(items: ScheduleItem[]): Record<string, ScheduleItem[]> {
    const grouped: Record<string, ScheduleItem[]> = {};
    for (const item of items) {
      (grouped[item.date] ??= []).push(item);
    }
    return grouped;
  }

  function teleworkForDate(key: string): TeleworkStatus {
    return data.telework[key] ?? { papa: false, mama: false };
  }

  function isScheduleItemVisible(item: ScheduleItem): boolean {
    const scopeType = scopeSelectValue(item.scopeType);
    if (scopeType === 'group') return showGroupSchedules;
    if (scopeType === 'world') return showWorldSchedules;
    return showPersonalSchedules;
  }

  function isMultiDayItemVisible(item: MultiDayScheduleItem): boolean {
    const scopeType = scopeSelectValue(item.scopeType);
    if (scopeType === 'group') return showGroupSchedules;
    if (scopeType === 'world') return showWorldSchedules;
    return showPersonalSchedules;
  }

  function isReadOnlyItem(item: ScheduleItem | MultiDayScheduleItem): boolean {
    return false;
  }

  function buildMultiDaySegments(items: MultiDayScheduleItem[], calendarGrid: Date[][]): Segment[] {
    const segments: Segment[] = [];
    const lanes = new Map<number, number>();
    const sorted = [...items].sort((a, b) =>
      `${normalizeDateKey(a.startDate)}-${normalizeDateKey(a.endDate)}`.localeCompare(`${normalizeDateKey(b.startDate)}-${normalizeDateKey(b.endDate)}`)
    );
    for (const item of sorted) {
      const itemEndDate = normalizeDateKey(item.endDate);
      const itemStartDate = normalizeDateKey(item.startDate);
      if (!itemStartDate || !itemEndDate) continue;
      const start = parseKey(item.startDate);
      const end = parseKey(item.endDate);
      for (let row = 0; row < calendarGrid.length; row += 1) {
        const rowStart = calendarGrid[row][0];
        const rowEnd = calendarGrid[row][6];
        const segmentStart = start > rowStart ? start : rowStart;
        const segmentEnd = end < rowEnd ? end : rowEnd;
        if (segmentStart <= segmentEnd) {
          const lane = lanes.get(row) ?? 0;
          lanes.set(row, lane + 1);
          if (lane < 4) {
            segments.push({
              id: item.id,
              row,
              startCol: Math.round((segmentStart.getTime() - rowStart.getTime()) / 86400000),
              endCol: Math.round((segmentEnd.getTime() - rowStart.getTime()) / 86400000),
              lane,
              text: item.text,
              color: item.color,
              arrow: item.arrow,
              isLast: dateKey(segmentEnd) === itemEndDate
            });
          }
        }
      }
    }
    return segments;
  }

  function groupSegmentsByRow(segments: Segment[], rowCount: number): Segment[][] {
    const grouped = Array.from({ length: rowCount }, () => [] as Segment[]);
    for (const segment of segments) {
      grouped[segment.row]?.push(segment);
    }
    return grouped;
  }

  function multiLaneCountForCell(segments: Segment[] | undefined, col: number): number {
    if (!segments?.length) return 0;
    const covering = segments.filter((segment) => segment.startCol <= col && col <= segment.endCol);
    if (!covering.length) return 0;
    return Math.max(...covering.map((segment) => segment.lane)) + 1;
  }

  function colorClass(color: ItemColor): string {
    return `item-${color}`;
  }

  function colorHex(color: ItemColor): string {
    if (color === 'red') return '#b91c1c';
    if (color === 'blue') return '#1d4ed8';
    return '#111111';
  }

  function dayClass(day: Date): string {
    const key = dateKey(day);
    const classes = ['day-cell'];
    if (day.getMonth() + 1 !== data.month) classes.push('outside');
    if (day.getDay() === 0 || holidays[key]) classes.push('sun-holiday');
    if (day.getDay() === 6) classes.push('sat');
    return classes.join(' ');
  }

  function weekdayText(weekday: number): string {
    const label = weekdays[weekday as keyof typeof weekdays];
    return `${label.jp} ${label.en}`;
  }

  function coerceColor(value: string): ItemColor {
    return value === 'red' || value === 'blue' ? value : 'black';
  }

  function buildPdfPayload(): CalendarData {
    return {
      ...data,
      scheduleItems: visibleScheduleItems,
      multiDayItems: visibleMultiDayItems
    };
  }

  async function generatePdfBlob(): Promise<Blob> {
    await saveCalendarNow();
    const response = await fetch(apiPath('pdf'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(buildPdfPayload())
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text);
    }
    return response.blob();
  }

  async function downloadPdf() {
    downloadBusy = true;
    statusMessage = '';
    try {
      const blob = await generatePdfBlob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `calendar-${data.year}-${pad(data.month)}.pdf`;
      link.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      statusMessage = error instanceof Error ? error.message : 'PDF の生成に失敗しました。';
    } finally {
      downloadBusy = false;
    }
  }

  async function previewPdf() {
    previewBusy = true;
    statusMessage = '';
    const previewWindow = window.open('', '_blank');
    if (!previewWindow) {
      previewBusy = false;
      statusMessage = 'PDF 表示用のタブを開けませんでした。ブラウザのポップアップ設定を確認してください。';
      return;
    }
    previewWindow.document.title = 'PDF 生成中';
    previewWindow.document.body.textContent = 'PDF を生成しています...';
    try {
      const blob = await generatePdfBlob();
      const url = URL.createObjectURL(blob);
      previewWindow.location.href = url;
      setTimeout(() => URL.revokeObjectURL(url), 60000);
    } catch (error) {
      previewWindow.close();
      statusMessage = error instanceof Error ? error.message : 'PDF の生成に失敗しました。';
    } finally {
      previewBusy = false;
    }
  }

  function exportJson() {
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `kazokucal-${data.year}-${pad(data.month)}.json`;
    link.click();
    URL.revokeObjectURL(url);
  }

  function openImportPicker() {
    importFileInput?.click();
  }

  async function importJson(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    statusMessage = '';
    importBusy = true;
    importResultOpen = false;
    try {
      const imported = normalizeData(JSON.parse(await file.text()) as Partial<CalendarData>);
      if (replaceOnImport) {
        commitData(imported);
        selectedDayKey = `${imported.year}-${pad(imported.month)}-01`;
        newMultiDay = blankMultiDayForDate(selectedDayKey);
      } else if (imported.year === data.year && imported.month === data.month) {
        commitData(mergeCalendarData(data, imported));
        selectedDayKey = `${imported.year}-${pad(imported.month)}-01`;
        newMultiDay = blankMultiDayForDate(selectedDayKey);
      } else {
        await saveCalendarNow();
        const existing = await fetchCalendar(imported.year, imported.month);
        commitData(mergeCalendarData(existing, imported));
        selectedDayKey = `${imported.year}-${pad(imported.month)}-01`;
        newMultiDay = blankMultiDayForDate(selectedDayKey);
      }
      await saveCalendarNow();
      importResultTitle = '読み込み完了';
      importResultMessage = `${imported.year}年${imported.month}月のデータを${replaceOnImport ? '入れ替え' : '読み込み'}ました。`;
    } catch (error) {
      importResultTitle = '読み込み失敗';
      importResultMessage = error instanceof Error ? error.message : 'JSON の読み込みに失敗しました。';
    } finally {
      importBusy = false;
      importResultOpen = true;
      input.value = '';
    }
  }

  async function changeCalendarPeriod(year: number, month: number) {
    try {
      await saveCalendarNow();
      await loadCalendar(year, month, false);
    } catch (error) {
      statusMessage = error instanceof Error ? error.message : 'カレンダーの保存または読み込みに失敗しました。';
    }
  }

  async function loadCalendar(year: number, month: number, initial: boolean) {
    loadBusy = true;
    statusMessage = '';
    try {
      const response = await fetch(apiPath(`calendar?year=${year}&month=${month}`));
      if (!response.ok) throw new Error(await response.text());
      data = normalizeData((await response.json()) as Partial<CalendarData>);
      selectedDayKey = `${data.year}-${pad(data.month)}-01`;
      newMultiDay = blankMultiDay();
      if (initial) ready = true;
    } catch (error) {
      data = normalizeData({ year, month });
      selectedDayKey = `${data.year}-${pad(data.month)}-01`;
      newMultiDay = blankMultiDay();
      statusMessage = error instanceof Error ? error.message : 'カレンダーの読み込みに失敗しました。';
      if (initial) ready = true;
    } finally {
      loadBusy = false;
    }
  }

  function scheduleSave() {
    if (!ready || loadBusy) return;
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      void saveCalendarNow().catch((error) => {
        statusMessage = error instanceof Error ? error.message : 'カレンダーの保存に失敗しました。';
      });
    }, 600);
  }

  async function saveCalendarNow() {
    if (!ready || loadBusy) return;
    if (saveTimer) {
      clearTimeout(saveTimer);
      saveTimer = undefined;
    }
    const response = await fetch(apiPath('calendar'), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
    if (!response.ok) {
      throw new Error(await response.text());
    }
  }

  function scopeSelectValue(scopeType: ScheduleScope | undefined): ScheduleScope {
    return scopeType === 'group' || scopeType === 'world' ? scopeType : 'personal';
  }

  function handleScheduleGroupChange(id: string, event: Event) {
    updateSchedule(id, { group: (event.currentTarget as HTMLSelectElement).value });
  }

  function handleMultiDayGroupChange(id: string, event: Event) {
    updateMultiDay(id, { group: (event.currentTarget as HTMLSelectElement).value });
  }

  function setNewScheduleScopeType(scopeType: ScheduleScope) {
    newScheduleScopeType = scopeType;
  }

  function setNewMultiDayScopeType(scopeType: ScheduleScope) {
    newMultiDayScopeType = scopeType;
  }
</script>

<main class="app-shell">
  <section class="toolbar">
    <div class="title-row">
      <svg class="brand-icon" viewBox="0 0 24 24" aria-hidden="true">
        <path
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          d="M17,7 L23,7 L23,23 L7,23 L7,19 M23,11 L17,11 M13,0 L13,3 M1,7 L17,7 M1,3 L17,3 L17,19 L1,19 L1,3 Z M5,0 L5,3 M4,11 L6,11 M8,11 L14,11 M4,15 L6,15 M8,15 L14,15"
        />
      </svg>
      <div>
        <div class="app-title-block">
          <h1>KazokuCal</h1>
          {#if appVersion || releaseCommitUrl}
            <div class="app-meta-line">
              {#if appVersion}
                <span class="app-version">{appVersion}</span>
              {/if}
              {#if releaseCommitUrl}
                <a
                  class="app-release-link"
                  href={releaseCommitUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={shortReleaseCommit ? `GitHub release commit ${shortReleaseCommit} を開く` : 'GitHub repository を開く'}
                  title={shortReleaseCommit ? `GitHub release commit ${shortReleaseCommit}` : 'GitHub repository'}
                >
                  <Github size={16} />
                  {#if shortReleaseCommit}
                    <span class="app-release-hash">{shortReleaseCommit}</span>
                  {/if}
                </a>
              {/if}
            </div>
          {/if}
        </div>
      </div>
    </div>

    <div class="controls">
      <label>
        年
        <input class="select-like year-input" type="number" min="1900" max="2100" value={data.year} on:change={(e) => updateYear(e.currentTarget.value)} />
      </label>
      <label>
        月
        <select class="select-like month-select" value={data.month} on:change={(e) => updateMonth(e.currentTarget.value)}>
          {#each Array.from({ length: 12 }, (_, i) => i + 1) as month}
            <option value={month}>{month}月</option>
          {/each}
        </select>
      </label>
      <label>
        週の始まり
        <select class="select-like weekstart-select" value={data.weekStartsOn} on:change={(e) => updateWeekStartsOn(e.currentTarget.value)}>
          <option value="monday">月曜日</option>
          <option value="sunday">日曜日</option>
        </select>
      </label>
      <div class="control-field">
        <span>表示</span>
        <div class="scope-toggle toolbar-scope-toggle" role="group" aria-label="表示する予定の種別">
          <button type="button" class:active={scopeVisibility.personal} on:click={() => toggleScopeVisibility('personal')} aria-pressed={scopeVisibility.personal} aria-label={`個人予定: ${scopeVisibility.personal ? 'ON' : 'OFF'}`} title={`個人予定: ${scopeVisibility.personal ? 'ON' : 'OFF'}`}>
            <UserRound size={16} />
          </button>
          {#if supportsGroupSchedules}
            <button type="button" class:active={scopeVisibility.group} on:click={() => toggleScopeVisibility('group')} aria-pressed={scopeVisibility.group} aria-label={`グループ予定: ${scopeVisibility.group ? 'ON' : 'OFF'}`} title={`グループ予定: ${scopeVisibility.group ? 'ON' : 'OFF'}`}>
              <UsersRound size={16} />
            </button>
          {/if}
          <button type="button" class:active={scopeVisibility.world} on:click={() => toggleScopeVisibility('world')} aria-pressed={scopeVisibility.world} aria-label={`共通予定: ${scopeVisibility.world ? 'ON' : 'OFF'}`} title={`共通予定: ${scopeVisibility.world ? 'ON' : 'OFF'}`}>
            <Globe size={16} />
          </button>
        </div>
      </div>
      <div class="control-field">
        <span>PDF</span>
        <div class="scope-toggle toolbar-action-toggle" role="group" aria-label="PDF 操作">
          <button
            type="button"
            class:active={previewBusy}
            on:click={previewPdf}
            disabled={pdfBusy}
            aria-label={previewBusy ? 'PDF 表示を生成中' : 'PDF を表示'}
            title={previewBusy ? 'PDF 表示を生成中' : 'PDF を表示'}
          >
            <Eye size={16} />
          </button>
          <button
            type="button"
            class:active={downloadBusy}
            on:click={downloadPdf}
            disabled={pdfBusy}
            aria-label={downloadBusy ? 'PDF ダウンロードを生成中' : 'PDF をダウンロード'}
            title={downloadBusy ? 'PDF をダウンロード' : 'PDF をダウンロード'}
          >
            <Download size={16} />
          </button>
        </div>
      </div>
      <div class="control-field">
        <span>データ</span>
        <div class="scope-toggle toolbar-action-toggle" role="group" aria-label="データ操作">
          <button
            type="button"
            on:click={exportJson}
            disabled={importBusy}
            aria-label="JSON を書き出し"
            title="JSON を書き出し"
          >
            <Download size={16} />
          </button>
          <button
            type="button"
            on:click={openImportPicker}
            disabled={importBusy}
            aria-label="JSON を読み込み"
            title="JSON を読み込み"
          >
            <Upload size={16} />
          </button>
          <button
            type="button"
            class:active={replaceOnImport}
            on:click={() => (replaceOnImport = !replaceOnImport)}
            aria-pressed={replaceOnImport}
            aria-label={`既存予定を消して入れ替え: ${replaceOnImport ? 'ON' : 'OFF'}`}
            title={`既存予定を消して入れ替え: ${replaceOnImport ? 'ON' : 'OFF'}`}
          >
            <Replace size={16} />
          </button>
          <input bind:this={importFileInput} class="toolbar-file-input" type="file" accept="application/json" on:change={importJson} />
        </div>
      </div>
      {#if userInfo.authenticated}
        <div class="control-field user-control" on:focusout={handleUserMenuFocusOut}>
          <span>ユーザ</span>
          <div class="scope-toggle toolbar-action-toggle user-menu-toggle" role="group" aria-label="ユーザ情報">
            <button
              type="button"
              class:active={userMenuOpen}
              aria-haspopup="menu"
              aria-expanded={userMenuOpen}
              aria-label={`ユーザ情報: ${userDisplayName}`}
              title={userDisplayName}
              on:click={() => (userMenuOpen = !userMenuOpen)}
            >
              <UserRound size={16} />
              <span>{userDisplayName}</span>
            </button>
          </div>
          {#if userMenuOpen}
            <div class="user-menu" role="menu">
              <dl>
                <div>
                  <dt>user</dt>
                  <dd>{userInfo.user ?? '-'}</dd>
                </div>
                <div>
                  <dt>email</dt>
                  <dd>{userInfo.email ?? '-'}</dd>
                </div>
                <div>
                  <dt>groups</dt>
                  <dd>{availableGroups.length > 0 ? availableGroups.join(', ') : '-'}</dd>
                </div>
              </dl>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </section>

  {#if statusMessage}
    <p class="status">{statusMessage}</p>
  {/if}

  <section class="calendar-preview" aria-label="カレンダープレビュー">
    <div class="calendar-title">
      <span>{data.year}</span>
      <strong>{data.month}</strong>
      <span>{monthNames[data.month]}</span>
    </div>
    <div class="weekday-grid">
      {#each weekdayOrder as weekday}
        <div class:weekday-sat={weekday === 6} class:weekday-sun={weekday === 0}>
          {weekdayText(weekday)}
        </div>
      {/each}
    </div>
    <div class="week-stack">
      {#each grid as week, rowIndex}
        {@const rowSegments = multiDaySegmentsByRow[rowIndex] ?? []}
        <div class="week-row">
          {#each week as day, colIndex}
            {@const key = dateKey(day)}
            {@const status = teleworkForDate(key)}
            {@const dayLaneCount = multiLaneCountForCell(rowSegments, colIndex)}
            <div
              class={dayClass(day)}
              class:has-multi-day={dayLaneCount > 0}
              style={`--day-multi-lanes: ${dayLaneCount};`}
              role="button"
              tabindex="0"
              on:click={() => openDay(day)}
              on:keydown={(event) => handleDayKeydown(event, day)}
            >
              <div class="day-head">
                <span class="day-number">{day.getDate()}</span>
                <span class="telework-labels">
                  {#if status.papa}<em>パパテレワーク</em>{/if}
                  {#if status.mama}<em>ママテレワーク</em>{/if}
                </span>
                {#if holidays[key]}
                  <span class="holiday-name">{holidays[key]}</span>
                {/if}
              </div>
              <div class="day-items">
                {#each visibleScheduleItemsByDate[key] ?? [] as item}
                  <span class={colorClass(item.color)}>{item.text}</span>
                {/each}
              </div>
            </div>
          {/each}
          {#if rowSegments.length > 0}
            <div class="multi-overlay">
              {#each rowSegments as segment}
                <div
                  class={`multi-bar ${colorClass(segment.color)}`}
                  role="button"
                  tabindex="0"
                  style={`left: calc(${(segment.startCol / 7) * 100}% + 18px); width: calc(${((segment.endCol - segment.startCol + 1) / 7) * 100}% - 36px); top: ${44 + segment.lane * 24}px;`}
                  on:click={() => openMultiDay(segment.id)}
                  on:keydown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      openMultiDay(segment.id);
                    }
                  }}
                >
                  <span>{segment.text}</span>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </section>
</main>

<Modal title={`${selectedDayKey} の編集`} bind:open={dayModalOpen} size="lg">
  <div class="day-editor">
    <div class="telework-editor">
      <Checkbox checked={selectedTelework.papa} on:change={(e) => setTelework('papa', e.currentTarget.checked)}>パパテレワーク</Checkbox>
      <Checkbox checked={selectedTelework.mama} on:change={(e) => setTelework('mama', e.currentTarget.checked)}>ママテレワーク</Checkbox>
    </div>

    <div class="editor-switch" role="tablist" aria-label="予定種別">
      <button class:active={dayEditorMode === 'schedule'} type="button" on:click={() => switchDayEditorMode('schedule')}>日予定</button>
      <button class:active={dayEditorMode === 'multi'} type="button" on:click={() => switchDayEditorMode('multi')}>期間予定</button>
    </div>

    {#if dayModalMessage}
      <p class="modal-status">{dayModalMessage}</p>
    {/if}

    {#if dayEditorMode === 'schedule'}
      <div class="schedule-add">
        <div class="schedule-add-main">
          <div class="copyable-input schedule-input-cell">
            <Input
              class="schedule-text-input"
              style={`color: ${colorHex(newScheduleColor)};`}
              placeholder="予定を入力"
              bind:value={newScheduleText}
              maxlength="120"
            />
            <button
              class:copied={copyFeedbackKey === 'new-schedule'}
              class="copy-button"
              type="button"
              aria-label={copyFeedbackKey === 'new-schedule' ? 'コピー済み' : '入力内容をコピー'}
              title={copyFeedbackKey === 'new-schedule' ? 'コピー済み' : '入力内容をコピー'}
              disabled={!newScheduleText}
              on:click={() => copyInputValue(newScheduleText, 'new-schedule')}
            >
              {#if copyFeedbackKey === 'new-schedule'}
                <Check size={16} />
              {:else}
                <Copy size={16} />
              {/if}
            </button>
          </div>
          <div class="color-palette schedule-color-cell" role="radiogroup" aria-label="追加する予定の色">
            {#each colorOptions as color}
              <button
                type="button"
                class={`color-swatch color-swatch-${color}`}
                class:active={newScheduleColor === color}
                role="radio"
                aria-checked={newScheduleColor === color}
                aria-label={colorLabels[color]}
                title={colorLabels[color]}
                on:click={() => (newScheduleColor = color)}
              >
                <span></span>
              </button>
            {/each}
          </div>
          {#if showScopeSelector}
            <div class="scope-toggle schedule-scope-cell" role="group" aria-label="予定の種別">
              {#each scopeToggleOptions() as option}
                <button type="button" class:active={newScheduleScopeType === option.value} on:click={() => setNewScheduleScopeType(option.value)} aria-pressed={newScheduleScopeType === option.value} aria-label={scopeOptionLabel(option.value)} title={scopeOptionLabel(option.value)}>
                  {#if option.value === 'personal'}
                    <UserRound size={16} />
                  {:else if option.value === 'group'}
                    <UsersRound size={16} />
                  {:else}
                    <Globe size={16} />
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
          <Button class="schedule-action-cell" color="dark" on:click={addSchedule}>
            <Plus size={16} class="mr-2" />
            追加
          </Button>
        </div>
        {#if newScheduleScopeType === 'group'}
          <div class="group-select-row">
            <select class="select-like compact-select" bind:value={newScheduleGroup} aria-label="追加する予定のグループ">
              {#each availableGroups as group}
                <option value={group}>{group}</option>
              {/each}
            </select>
          </div>
        {/if}
      </div>

      <div class="schedule-list">
        {#each selectedItems as item (item.id)}
          <div class:dragging={draggingScheduleId === item.id} class:drag-over={dragOverScheduleId === item.id} class="schedule-row" data-schedule-id={item.id}>
            <button
              class="drag-handle"
              type="button"
              aria-label="予定の順番を変更"
              title="ドラッグして順番を変更"
              disabled={selectedItems.length < 2 || isReadOnlyItem(item)}
              on:pointerdown={(event) => startScheduleDrag(event, item.id)}
              on:keydown={(event) => handleScheduleHandleKeydown(event, item.id)}
            >
              <GripVertical size={18} />
            </button>
            <div class="schedule-edit-grid">
              <div class="copyable-input schedule-input-cell">
                <Input
                  class="schedule-text-input"
                  style={`color: ${colorHex(item.color)};`}
                  value={item.text}
                  maxlength="120"
                  disabled={isReadOnlyItem(item)}
                  on:input={(e) => updateSchedule(item.id, { text: e.currentTarget.value })}
                />
                <button
                  class:copied={copyFeedbackKey === `schedule-${item.id}`}
                  class="copy-button"
                  type="button"
                  aria-label={copyFeedbackKey === `schedule-${item.id}` ? 'コピー済み' : '入力内容をコピー'}
                  title={copyFeedbackKey === `schedule-${item.id}` ? 'コピー済み' : '入力内容をコピー'}
                  disabled={!item.text}
                  on:click={() => copyInputValue(item.text, `schedule-${item.id}`)}
                >
                  {#if copyFeedbackKey === `schedule-${item.id}`}
                    <Check size={16} />
                  {:else}
                    <Copy size={16} />
                  {/if}
                </button>
              </div>
              <div class="color-palette schedule-color-cell" role="radiogroup" aria-label="予定の色" aria-disabled={isReadOnlyItem(item)}>
                {#each colorOptions as color}
                  <button
                    type="button"
                    class={`color-swatch color-swatch-${color}`}
                    class:active={item.color === color}
                    role="radio"
                    aria-checked={item.color === color}
                    aria-label={colorLabels[color]}
                    title={colorLabels[color]}
                    disabled={isReadOnlyItem(item)}
                    on:click={() => updateSchedule(item.id, { color })}
                  >
                    <span></span>
                  </button>
                {/each}
              </div>
              {#if showScopeSelector && !isReadOnlyItem(item)}
                <div class="scope-toggle schedule-scope-cell" role="group" aria-label="予定の種別">
                  {#each scopeToggleOptions() as option}
                    <button type="button" class:active={scopeSelectValue(item.scopeType) === option.value} on:click={() => updateSchedule(item.id, { scopeType: option.value })} aria-pressed={scopeSelectValue(item.scopeType) === option.value} aria-label={scopeOptionLabel(option.value)} title={scopeOptionLabel(option.value)}>
                      {#if option.value === 'personal'}
                        <UserRound size={16} />
                      {:else if option.value === 'group'}
                        <UsersRound size={16} />
                      {:else}
                        <Globe size={16} />
                      {/if}
                    </button>
                  {/each}
                </div>
              {/if}
              {#if !isReadOnlyItem(item)}
                <Button class="schedule-delete-cell" color="red" on:click={() => deleteSchedule(item.id)} aria-label="予定を削除">
                  <Trash2 size={16} />
                </Button>
              {/if}
            </div>
            {#if item.scopeType === 'group'}
              <div class="group-select-row">
                <select
                  class="select-like compact-select"
                  value={item.group}
                  aria-label="予定のグループ"
                  on:change={(e) => handleScheduleGroupChange(item.id, e)}
                >
                  {#each availableGroups as group}
                    <option value={group}>{group}</option>
                  {/each}
                </select>
              </div>
            {/if}
          </div>
        {/each}
        {#if selectedItems.length === 0}
          <p class="empty">この日の予定はまだありません。</p>
        {/if}
      </div>
    {:else}
      <div class="multi-editor-form">
        {#if showScopeSelector}
          <div class="scope-toggle" role="group" aria-label="期間予定の種別">
            {#each scopeToggleOptions() as option}
              <button type="button" class:active={newMultiDayScopeType === option.value} on:click={() => setNewMultiDayScopeType(option.value)} aria-pressed={newMultiDayScopeType === option.value} aria-label={scopeOptionLabel(option.value)} title={scopeOptionLabel(option.value)}>
                {#if option.value === 'personal'}
                  <UserRound size={16} />
                {:else if option.value === 'group'}
                  <UsersRound size={16} />
                {:else}
                  <Globe size={16} />
                {/if}
              </button>
            {/each}
          </div>
          {#if newMultiDayScopeType === 'group'}
            <div class="group-select-row">
              <select class="select-like compact-select" bind:value={newMultiDayGroup} aria-label="追加する期間予定のグループ">
                {#each availableGroups as group}
                  <option value={group}>{group}</option>
                {/each}
              </select>
            </div>
          {/if}
        {/if}
        <div class="field">
          <span>開始日</span>
          <div class="copyable-input">
            <Input type="date" bind:value={newMultiDay.startDate} />
            <button
              class:copied={copyFeedbackKey === 'new-multi-start'}
              class="copy-button"
              type="button"
              aria-label={copyFeedbackKey === 'new-multi-start' ? 'コピー済み' : '入力内容をコピー'}
              title={copyFeedbackKey === 'new-multi-start' ? 'コピー済み' : '入力内容をコピー'}
              disabled={!newMultiDay.startDate}
              on:click={() => copyInputValue(newMultiDay.startDate, 'new-multi-start')}
            >
              {#if copyFeedbackKey === 'new-multi-start'}
                <Check size={16} />
              {:else}
                <Copy size={16} />
              {/if}
            </button>
          </div>
        </div>
        <div class="field">
          <span>終了日</span>
          <div class="copyable-input">
            <Input type="date" bind:value={newMultiDay.endDate} />
            <button
              class:copied={copyFeedbackKey === 'new-multi-end'}
              class="copy-button"
              type="button"
              aria-label={copyFeedbackKey === 'new-multi-end' ? 'コピー済み' : '入力内容をコピー'}
              title={copyFeedbackKey === 'new-multi-end' ? 'コピー済み' : '入力内容をコピー'}
              disabled={!newMultiDay.endDate}
              on:click={() => copyInputValue(newMultiDay.endDate, 'new-multi-end')}
            >
              {#if copyFeedbackKey === 'new-multi-end'}
                <Check size={16} />
              {:else}
                <Copy size={16} />
              {/if}
            </button>
          </div>
        </div>
        <div class="field">
          <span>予定名</span>
          <div class="copyable-input">
            <Input placeholder="予定名（空欄可）" bind:value={newMultiDay.text} maxlength="80" />
            <button
              class:copied={copyFeedbackKey === 'new-multi-text'}
              class="copy-button"
              type="button"
              aria-label={copyFeedbackKey === 'new-multi-text' ? 'コピー済み' : '入力内容をコピー'}
              title={copyFeedbackKey === 'new-multi-text' ? 'コピー済み' : '入力内容をコピー'}
              disabled={!newMultiDay.text}
              on:click={() => copyInputValue(newMultiDay.text, 'new-multi-text')}
            >
              {#if copyFeedbackKey === 'new-multi-text'}
                <Check size={16} />
              {:else}
                <Copy size={16} />
              {/if}
            </button>
          </div>
        </div>
        <div class="field">
          <span>色</span>
          <div class="color-palette" role="radiogroup" aria-label="期間予定の色">
            {#each colorOptions as color}
              <button
                type="button"
                class={`color-swatch color-swatch-${color}`}
                class:active={newMultiDay.color === color}
                role="radio"
                aria-checked={newMultiDay.color === color}
                aria-label={colorLabels[color]}
                title={colorLabels[color]}
                on:click={() => (newMultiDay.color = color)}
              >
                <span></span>
              </button>
            {/each}
          </div>
        </div>
        <Button color="dark" on:click={addMultiDay}>
          <Plus size={16} class="mr-2" />
          期間予定を追加
        </Button>
      </div>
    {/if}
  </div>
</Modal>

<Modal title="期間予定の編集" bind:open={multiDayModalOpen} size="md">
  {#if selectedMultiDayItem}
    <div class="multi-editor-form">
      {#if showScopeSelector && !isReadOnlyItem(selectedMultiDayItem)}
        <div class="scope-toggle" role="group" aria-label="期間予定の種別">
          {#each scopeToggleOptions() as option}
            <button type="button" class:active={scopeSelectValue(selectedMultiDayItem.scopeType) === option.value} on:click={() => updateMultiDay(selectedMultiDayItem.id, { scopeType: option.value })} aria-pressed={scopeSelectValue(selectedMultiDayItem.scopeType) === option.value} aria-label={scopeOptionLabel(option.value)} title={scopeOptionLabel(option.value)}>
              {#if option.value === 'personal'}
                <UserRound size={16} />
              {:else if option.value === 'group'}
                <UsersRound size={16} />
              {:else}
                <Globe size={16} />
              {/if}
            </button>
          {/each}
        </div>
        {#if selectedMultiDayItem.scopeType === 'group'}
          <div class="group-select-row">
            <select
              class="select-like compact-select"
              value={selectedMultiDayItem.group}
              aria-label="期間予定のグループ"
              on:change={(e) => handleMultiDayGroupChange(selectedMultiDayItem.id, e)}
            >
              {#each availableGroups as group}
                <option value={group}>{group}</option>
              {/each}
            </select>
          </div>
        {/if}
      {/if}
      <div class="field">
        <span>開始日</span>
        <div class="copyable-input">
          <Input type="date" value={selectedMultiDayItem.startDate} disabled={isReadOnlyItem(selectedMultiDayItem)} on:change={(e) => updateMultiDay(selectedMultiDayItem.id, { startDate: e.currentTarget.value })} />
          <button
            class:copied={copyFeedbackKey === `multi-start-${selectedMultiDayItem.id}`}
            class="copy-button"
            type="button"
            aria-label={copyFeedbackKey === `multi-start-${selectedMultiDayItem.id}` ? 'コピー済み' : '入力内容をコピー'}
            title={copyFeedbackKey === `multi-start-${selectedMultiDayItem.id}` ? 'コピー済み' : '入力内容をコピー'}
            disabled={!selectedMultiDayItem.startDate}
            on:click={() => copyInputValue(selectedMultiDayItem.startDate, `multi-start-${selectedMultiDayItem.id}`)}
          >
            {#if copyFeedbackKey === `multi-start-${selectedMultiDayItem.id}`}
              <Check size={16} />
            {:else}
              <Copy size={16} />
            {/if}
          </button>
        </div>
      </div>
      <div class="field">
        <span>終了日</span>
        <div class="copyable-input">
          <Input type="date" value={selectedMultiDayItem.endDate} disabled={isReadOnlyItem(selectedMultiDayItem)} on:change={(e) => updateMultiDay(selectedMultiDayItem.id, { endDate: e.currentTarget.value })} />
          <button
            class:copied={copyFeedbackKey === `multi-end-${selectedMultiDayItem.id}`}
            class="copy-button"
            type="button"
            aria-label={copyFeedbackKey === `multi-end-${selectedMultiDayItem.id}` ? 'コピー済み' : '入力内容をコピー'}
            title={copyFeedbackKey === `multi-end-${selectedMultiDayItem.id}` ? 'コピー済み' : '入力内容をコピー'}
            disabled={!selectedMultiDayItem.endDate}
            on:click={() => copyInputValue(selectedMultiDayItem.endDate, `multi-end-${selectedMultiDayItem.id}`)}
          >
            {#if copyFeedbackKey === `multi-end-${selectedMultiDayItem.id}`}
              <Check size={16} />
            {:else}
              <Copy size={16} />
            {/if}
          </button>
        </div>
      </div>
      <div class="field">
        <span>予定名</span>
        <div class="copyable-input">
          <Input value={selectedMultiDayItem.text} placeholder="予定名" maxlength="80" disabled={isReadOnlyItem(selectedMultiDayItem)} on:input={(e) => updateMultiDay(selectedMultiDayItem.id, { text: e.currentTarget.value })} />
          <button
            class:copied={copyFeedbackKey === `multi-text-${selectedMultiDayItem.id}`}
            class="copy-button"
            type="button"
            aria-label={copyFeedbackKey === `multi-text-${selectedMultiDayItem.id}` ? 'コピー済み' : '入力内容をコピー'}
            title={copyFeedbackKey === `multi-text-${selectedMultiDayItem.id}` ? 'コピー済み' : '入力内容をコピー'}
            disabled={!selectedMultiDayItem.text}
            on:click={() => copyInputValue(selectedMultiDayItem.text, `multi-text-${selectedMultiDayItem.id}`)}
          >
            {#if copyFeedbackKey === `multi-text-${selectedMultiDayItem.id}`}
              <Check size={16} />
            {:else}
              <Copy size={16} />
            {/if}
          </button>
        </div>
      </div>
      <div class="field">
        <span>色</span>
        <div class="color-palette" role="radiogroup" aria-label="期間予定の色" aria-disabled={isReadOnlyItem(selectedMultiDayItem)}>
          {#each colorOptions as color}
            <button
              type="button"
              class={`color-swatch color-swatch-${color}`}
              class:active={selectedMultiDayItem.color === color}
              role="radio"
              aria-checked={selectedMultiDayItem.color === color}
              aria-label={colorLabels[color]}
              title={colorLabels[color]}
              disabled={isReadOnlyItem(selectedMultiDayItem)}
              on:click={() => updateMultiDay(selectedMultiDayItem.id, { color })}
            >
              <span></span>
            </button>
          {/each}
        </div>
      </div>
      <div class="modal-actions">
        {#if !isReadOnlyItem(selectedMultiDayItem)}
          <Button color="red" on:click={() => deleteMultiDay(selectedMultiDayItem.id)} aria-label="期間予定を削除">
            <Trash2 size={16} class="mr-2" />
            削除
          </Button>
        {/if}
      </div>
    </div>
  {/if}
</Modal>

{#if importBusy}
  <div class="blocking-overlay" role="alert" aria-live="assertive" aria-busy="true">
    <div class="blocking-dialog">
      <div class="spinner" aria-hidden="true"></div>
      <div>
        <strong>JSON を読み込んでいます</strong>
        <p>処理が完了するまでそのままお待ちください。</p>
      </div>
    </div>
  </div>
{/if}

<Modal title={importResultTitle} bind:open={importResultOpen} size="sm">
  <div class="result-dialog">
    <p>{importResultMessage}</p>
    <Button color="dark" on:click={() => (importResultOpen = false)}>閉じる</Button>
  </div>
</Modal>

<style>
  .app-shell {
    max-width: 1480px;
    margin: 0 auto;
    padding: 20px;
  }

  .toolbar,
  .panel {
    background: #ffffff;
    border: 1px solid #d1d5db;
    border-radius: 8px;
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 20px;
    padding: 16px;
  }

  .title-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  h1,
  p {
    margin: 0;
  }

  h1 {
    font-size: 22px;
    font-weight: 800;
    line-height: 1;
  }

  .brand-icon {
    width: 30px;
    height: 30px;
    color: #111827;
    flex: 0 0 auto;
  }

  .app-title-block {
    display: inline-flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 5px;
  }

  .app-meta-line {
    display: inline-flex;
    align-items: flex-end;
    gap: 8px;
  }

  .app-version,
  .app-release-link {
    color: #6b7280;
    font-size: 16px;
    font-weight: 700;
    letter-spacing: 0;
    line-height: 1;
  }

  .app-release-link {
    display: inline-flex;
    align-items: flex-end;
    gap: 4px;
    text-decoration: none;
    opacity: 0.78;
    transition:
      color 0.16s ease,
      opacity 0.16s ease;
  }

  .app-release-link:hover,
  .app-release-link:focus-visible {
    color: #111827;
    opacity: 1;
  }

  .app-release-link:focus-visible {
    border-radius: 4px;
    outline: 2px solid rgb(37 99 235 / 35%);
    outline-offset: 2px;
  }

  .app-release-link :global(svg) {
    width: 16px;
    height: 16px;
  }

  .app-release-hash {
    font-size: 10px;
  }

  .controls,
  .multi-form,
  .multi-row,
  .schedule-add,
  .schedule-row {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .controls {
    align-items: flex-end;
  }

  .controls > label {
    display: grid;
    gap: 4px;
    color: #374151;
    font-size: 12px;
    font-weight: 700;
  }

  .control-field {
    display: grid;
    gap: 4px;
    color: #374151;
    font-size: 12px;
    font-weight: 700;
  }

  .select-like {
    height: 42px;
    min-width: 88px;
    border: 1px solid #d1d5db;
    border-radius: 6px;
    background: #ffffff;
    color: #111827;
    padding: 0 10px;
    font-size: 14px;
  }

  .year-input {
    width: 98px;
  }

  .month-select {
    min-width: 90px;
  }

  .weekstart-select {
    min-width: 106px;
  }

  .compact-select {
    min-width: 140px;
  }

  .scope-toggle {
    display: inline-flex;
    gap: 2px;
    width: fit-content;
    max-width: 100%;
    border: 1px solid #d1d5db;
    border-radius: 7px;
    background: #f3f4f6;
    padding: 2px;
  }

  .scope-toggle button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 42px;
    min-width: 42px;
    height: 42px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    padding: 0;
    color: #374151;
    font-size: 14px;
    font-weight: 800;
    cursor: pointer;
    transition:
      background-color 0.16s ease,
      color 0.16s ease,
      box-shadow 0.16s ease,
      transform 0.16s ease;
  }

  .scope-toggle button.active {
    background: #ffffff;
    color: #111827;
    box-shadow: 0 1px 2px rgb(17 24 39 / 12%);
  }

  .toolbar-scope-toggle button.active {
    background: #2563eb;
    color: #ffffff;
    box-shadow: none;
  }

  .toolbar-action-toggle button {
    gap: 4px;
    width: auto;
    min-width: 42px;
    padding: 0 12px;
  }

  .toolbar-action-toggle button:hover:not(:disabled):not(.active) {
    background: #e5e7eb;
    color: #111827;
  }

  .toolbar-action-toggle button.active {
    background: #2563eb;
    color: #ffffff;
    box-shadow: none;
  }

  .toolbar-action-toggle button.active:hover:not(:disabled) {
    background: #1d4ed8;
  }

  .toolbar-action-toggle button:disabled {
    cursor: default;
    opacity: 0.5;
  }

  .user-control {
    position: relative;
  }

  .user-menu-toggle button {
    max-width: 180px;
    gap: 6px;
  }

  .user-menu-toggle button span {
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .user-menu {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 20;
    min-width: 220px;
    border: 1px solid #d1d5db;
    border-radius: 7px;
    background: #ffffff;
    padding: 10px 12px;
    box-shadow: 0 10px 24px rgb(17 24 39 / 14%);
    color: #111827;
    font-size: 12px;
    font-weight: 600;
  }

  .user-menu dl {
    display: grid;
    gap: 8px;
    margin: 0;
  }

  .user-menu dl > div {
    display: grid;
    grid-template-columns: 52px minmax(0, 1fr);
    gap: 10px;
  }

  .user-menu dt {
    color: #6b7280;
    font-weight: 700;
  }

  .user-menu dd {
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
  }

  .scope-toggle button :global(svg) {
    width: 16px;
    height: 16px;
  }

  .scope-toggle button:focus-visible {
    outline: 2px solid rgb(37 99 235 / 35%);
    outline-offset: 1px;
  }

  .status {
    margin-top: 12px;
    padding: 10px 12px;
    border: 1px solid #fecaca;
    border-radius: 6px;
    background: #fef2f2;
    color: #991b1b;
    font-size: 13px;
  }

  .modal-status {
    padding: 10px 12px;
    border: 1px solid #fecaca;
    border-radius: 6px;
    background: #fef2f2;
    color: #991b1b;
    font-size: 13px;
    font-weight: 700;
  }

  .calendar-preview {
    margin-top: 16px;
    background: #fffdf7;
    border: 1px solid #c7c2b8;
    padding: 24px 18px 18px;
    box-shadow: 0 8px 20px rgb(17 24 39 / 8%);
  }

  .calendar-title {
    display: flex;
    justify-content: center;
    align-items: baseline;
    gap: 34px;
    height: 54px;
    font-weight: 800;
  }

  .calendar-title span {
    font-size: 28px;
  }

  .calendar-title strong {
    font-size: 52px;
    line-height: 1;
  }

  .weekday-grid,
  .week-row {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
  }

  .weekday-grid > div {
    border: 1px solid #1f2937;
    border-bottom: 0;
    border-left: 0;
    background: #e5e7eb;
    padding: 7px 4px;
    text-align: center;
    font-weight: 800;
  }

  .weekday-grid > div:first-child {
    border-left: 1px solid #1f2937;
  }

  .weekday-grid .weekday-sat {
    background: #476b9e;
    color: #ffffff;
  }

  .weekday-grid .weekday-sun {
    background: #a25a4b;
    color: #ffffff;
  }

  .week-row {
    position: relative;
    isolation: isolate;
    min-height: clamp(104px, 12vw, 136px);
  }

  .day-cell {
    display: block;
    position: relative;
    z-index: 1;
    min-width: 0;
    width: 100%;
    min-height: clamp(104px, 12vw, 136px);
    align-self: stretch;
    border: 1px solid #1f2937;
    border-top: 0;
    border-left: 0;
    background: #fffdf7;
    padding: 7px 7px 5px;
    text-align: left;
    vertical-align: top;
    overflow: hidden;
    cursor: pointer;
  }

  .day-cell:first-child {
    border-left: 1px solid #1f2937;
  }

  .day-cell.sat {
    background: #f7fbff;
  }

  .day-cell.sun-holiday {
    background: #fff8f8;
  }

  .day-cell.outside {
    background: #f3f4f6;
  }

  .day-head {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    min-height: 31px;
  }

  .day-number {
    color: #111827;
    font-size: 30px;
    font-weight: 900;
    line-height: 1;
  }

  .sat .day-number {
    color: #365f91;
  }

  .sun-holiday .day-number,
  .holiday-name {
    color: #9f2f2d;
  }

  .day-cell.outside .day-number,
  .day-cell.outside .holiday-name {
    color: #9ca3af;
  }

  .telework-labels {
    display: grid;
    gap: 1px;
    padding-top: 2px;
    color: #374151;
    font-size: 11px;
    font-style: normal;
    font-weight: 700;
    line-height: 1.2;
  }

  .telework-labels em {
    font-style: normal;
  }

  .holiday-name {
    min-width: 0;
    padding-top: 2px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: 12px;
    font-weight: 800;
    line-height: 1.2;
  }

  .day-items {
    display: grid;
    gap: 2px;
    margin-top: 8px;
    font-size: 13px;
    line-height: 1.25;
  }

  .day-cell.has-multi-day .day-items {
    margin-top: calc(8px + var(--day-multi-lanes) * 24px);
  }

  .day-items span {
    overflow-wrap: anywhere;
  }

  .item-black {
    color: #111111;
    border-color: #111111;
  }

  .item-red {
    color: #b91c1c;
    border-color: #b91c1c;
  }

  .item-blue {
    color: #1d4ed8;
    border-color: #1d4ed8;
  }

  .multi-overlay {
    position: absolute;
    inset: 0;
    z-index: 10;
    pointer-events: none;
  }

  .multi-bar {
    position: absolute;
    display: flex;
    align-items: center;
    justify-content: center;
    height: 20px;
    min-width: 28px;
    border: 1px solid currentColor;
    border-radius: 3px;
    background: #e5e7eb;
    cursor: pointer;
    pointer-events: auto;
  }

  .multi-bar.item-red {
    background: #fee2e2;
  }

  .multi-bar.item-blue {
    background: #dbeafe;
  }

  .multi-bar span {
    min-width: 0;
    max-width: calc(100% - 10px);
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: #111827;
    font-size: 12px;
    font-weight: 700;
    line-height: 1;
  }

  .multi-bar:focus-visible {
    outline: 3px solid #2563eb;
    outline-offset: 2px;
  }

  .panel {
    padding: 16px;
  }

  .multi-form,
  .multi-row {
    align-items: center;
  }

  .multi-list {
    display: grid;
    gap: 8px;
    margin-top: 12px;
  }

  .multi-row {
    padding: 8px;
    border: 1px solid #e5e7eb;
    border-radius: 6px;
    background: #f9fafb;
  }

  .editor-switch {
    display: inline-flex;
    gap: 2px;
    width: fit-content;
    border: 1px solid #d1d5db;
    border-radius: 7px;
    background: #f3f4f6;
    padding: 2px;
  }

  .editor-switch button {
    min-width: 92px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    padding: 8px 12px;
    color: #374151;
    font-size: 14px;
    font-weight: 800;
    cursor: pointer;
  }

  .editor-switch button.active {
    background: #ffffff;
    color: #111827;
    box-shadow: 0 1px 2px rgb(17 24 39 / 12%);
  }

  .multi-editor-form {
    display: grid;
    gap: 12px;
  }

  .multi-editor-form .field {
    display: grid;
    gap: 5px;
    color: #374151;
    font-size: 12px;
    font-weight: 800;
  }

  .multi-editor-form .select-like {
    width: 100%;
  }

  .color-palette {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    width: fit-content;
    min-height: 42px;
    border: 1px solid #d1d5db;
    border-radius: 6px;
    background: #ffffff;
    padding: 5px;
  }

  .color-swatch {
    display: inline-grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border: 2px solid transparent;
    border-radius: 6px;
    background: transparent;
    padding: 0;
    cursor: pointer;
  }

  .color-swatch span {
    width: 18px;
    height: 18px;
    border: 1px solid rgb(17 24 39 / 20%);
    border-radius: 4px;
  }

  .color-swatch.active {
    border-color: #111827;
    background: #f3f4f6;
  }

  .color-swatch:focus-visible {
    outline: 3px solid rgb(37 99 235 / 35%);
    outline-offset: 2px;
  }

  .color-swatch-black span {
    background: #111111;
  }

  .color-swatch-red span {
    background: #b91c1c;
  }

  .color-swatch-blue span {
    background: #1d4ed8;
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
  }

  .blocking-overlay {
    position: fixed;
    inset: 0;
    z-index: 9999;
    display: grid;
    place-items: center;
    background: rgb(17 24 39 / 48%);
    padding: 20px;
  }

  .blocking-dialog {
    display: flex;
    align-items: center;
    gap: 14px;
    width: min(420px, 100%);
    border: 1px solid #d1d5db;
    border-radius: 8px;
    background: #ffffff;
    padding: 18px;
    box-shadow: 0 18px 40px rgb(17 24 39 / 24%);
  }

  .blocking-dialog strong {
    display: block;
    color: #111827;
    font-size: 16px;
    font-weight: 800;
  }

  .blocking-dialog p,
  .result-dialog p {
    color: #4b5563;
    font-size: 14px;
  }

  .spinner {
    width: 28px;
    height: 28px;
    border: 3px solid #d1d5db;
    border-top-color: #1f2937;
    border-radius: 999px;
    animation: spin 0.8s linear infinite;
    flex: 0 0 auto;
  }

  .result-dialog {
    display: grid;
    gap: 16px;
  }

  .result-dialog :global(button) {
    justify-self: end;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .toolbar-file-input {
    display: none;
  }

  .day-editor,
  .schedule-list {
    display: grid;
    gap: 14px;
  }

  .telework-editor {
    display: flex;
    gap: 18px;
    flex-wrap: wrap;
  }

  .schedule-add {
    display: grid;
    gap: 8px;
    width: 100%;
  }

  .schedule-add-main,
  .schedule-edit-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto auto;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-width: 0;
  }

  .schedule-row {
    display: grid;
    grid-template-columns: 34px minmax(0, 1fr);
    align-items: start;
    gap: 8px;
    padding: 8px;
    border: 1px solid #e5e7eb;
    border-radius: 6px;
    background: #f9fafb;
  }

  .schedule-input-cell {
    min-width: 0;
    width: 100%;
  }

  .copyable-input {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    min-width: 0;
    width: 100%;
  }

  .schedule-input-cell :global(.relative),
  .schedule-input-cell :global(input) {
    width: 100%;
  }

  .copyable-input :global(.relative),
  .copyable-input :global(input) {
    width: 100%;
  }

  .copy-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 42px;
    min-width: 42px;
    height: 42px;
    border: 1px solid #d1d5db;
    border-radius: 6px;
    background: #ffffff;
    color: #4b5563;
    padding: 0;
    cursor: pointer;
    transition:
      border-color 0.16s ease,
      background-color 0.16s ease,
      color 0.16s ease;
  }

  .copy-button:hover:not(:disabled) {
    border-color: #9ca3af;
    background: #f9fafb;
    color: #111827;
  }

  .copy-button.copied {
    border-color: #16a34a;
    background: #f0fdf4;
    color: #15803d;
  }

  .copy-button:disabled {
    cursor: default;
    opacity: 0.45;
  }

  .copy-button:focus-visible {
    outline: 3px solid rgb(37 99 235 / 35%);
    outline-offset: 2px;
  }

  .schedule-color-cell {
    justify-self: start;
  }

  .schedule-scope-cell {
    justify-self: start;
  }

  :global(.schedule-action-cell),
  :global(.schedule-delete-cell) {
    justify-self: end;
  }

  .group-select-row {
    width: 100%;
  }

  .schedule-row .group-select-row {
    grid-column: 2;
  }

  .group-select-row .select-like {
    width: min(240px, 100%);
  }

  .schedule-row.dragging {
    border-color: #60a5fa;
    background: #eff6ff;
  }

  .schedule-row.drag-over {
    box-shadow: inset 0 2px 0 #2563eb;
  }

  .drag-handle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 30px;
    height: 42px;
    border: 1px solid #d1d5db;
    border-radius: 6px;
    background: #ffffff;
    color: #4b5563;
    cursor: grab;
    touch-action: none;
  }

  .drag-handle:active {
    cursor: grabbing;
  }

  .drag-handle:disabled {
    cursor: default;
    opacity: 0.35;
  }

  .schedule-row .color-palette,
  .schedule-add .color-palette {
    min-width: 0;
    width: fit-content;
    justify-content: center;
  }

  .empty {
    color: #6b7280;
    font-size: 13px;
  }

  @media (max-width: 980px) {
    .toolbar {
      display: block;
    }

    .controls {
      margin-top: 14px;
      gap: 8px;
    }

    .calendar-preview {
      overflow-x: auto;
    }

    .weekday-grid,
    .week-stack {
      min-width: 920px;
    }

    .schedule-add-main,
    .schedule-edit-grid {
      display: flex;
      flex-wrap: wrap;
      align-items: stretch;
    }

    .schedule-input-cell {
      order: 1;
      flex: 1 0 100%;
    }

    .schedule-color-cell {
      order: 2;
      flex: 0 0 auto;
      justify-self: start;
    }

    .schedule-scope-cell {
      order: 4;
      flex: 1 0 100%;
      justify-self: stretch;
    }

    .schedule-scope-cell.scope-toggle {
      width: 100%;
    }

    .schedule-scope-cell.scope-toggle button {
      flex: 1 1 0;
      width: auto;
    }

    :global(.schedule-action-cell),
    :global(.schedule-delete-cell) {
      order: 3;
      margin-left: auto;
      align-self: center;
    }

    :global(.schedule-delete-cell),
    :global(.schedule-action-cell) {
      width: auto;
      min-width: 42px;
      height: 42px;
    }

    .drag-handle {
      height: 100%;
      min-height: 92px;
    }
  }
</style>
