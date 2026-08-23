import type { CategoryWithUsage } from '../db/repo';

/**
 * Turning the flat category table back into the two-level tree the server
 * keeps, and ordering it by what actually gets used.
 *
 * The cache is deliberately flat — a transaction refers to one category by id,
 * so storing a tree would buy nothing — but a picker showing forty unrelated
 * names in one run-on list is unusable. The parent is still on every row, so
 * the tree can be rebuilt here, at the only place that needs it.
 */

/** Primary categories carry this as their parent, per the server. */
export const LEVEL_ONE_PARENT_ID = '0';

export interface CategoryGroup {
    /**
     * The primary category whose children these are, or null for the trailing
     * group of primaries that have no children of their own.
     */
    parentId: string | null;
    /** Section heading. Null for the trailing group, which has no one name. */
    name: string | null;
    categories: CategoryWithUsage[];
    /** Uses summed across the group, which is what orders the groups. */
    uses: number;
}

/**
 * Most used first, then the order the server gave, then by name so the result
 * never depends on which order SQLite happened to return rows in.
 */
function byUsageThenOrder(a: CategoryWithUsage, b: CategoryWithUsage): number {
    return b.uses - a.uses || a.displayOrder - b.displayOrder || a.name.localeCompare(b.name);
}

/**
 * Groups categories of one type under their primary category.
 *
 * Three kinds of row have to land somewhere sensible:
 *
 * - a child, which goes under its primary;
 * - a primary with children, which becomes the heading and is not itself
 *   selectable — picking "Food & Drink" when "Groceries" exists is never what
 *   was meant;
 * - a primary with no children, which stays selectable and collects into one
 *   unlabelled group at the end, alongside any orphan whose primary is hidden
 *   or missing. Nothing is dropped for having an awkward shape.
 */
export function groupCategories(categories: CategoryWithUsage[], type: number): CategoryGroup[] {
    const ofType = categories.filter((category) => category.type === type);

    const primaries = ofType.filter((category) => category.parentId === LEVEL_ONE_PARENT_ID);
    const primaryById = new Map(primaries.map((category) => [category.id, category]));

    const childrenByParent = new Map<string, CategoryWithUsage[]>();
    const loose: CategoryWithUsage[] = [];

    for (const category of ofType) {
        if (category.parentId === LEVEL_ONE_PARENT_ID) {
            continue;
        }

        if (!primaryById.has(category.parentId)) {
            loose.push(category);
            continue;
        }

        const siblings = childrenByParent.get(category.parentId);

        if (siblings) {
            siblings.push(category);
        } else {
            childrenByParent.set(category.parentId, [category]);
        }
    }

    const groups: CategoryGroup[] = [];

    for (const primary of primaries) {
        const children = childrenByParent.get(primary.id);

        if (!children?.length) {
            loose.push(primary);
            continue;
        }

        const sorted = [...children].sort(byUsageThenOrder);

        groups.push({
            parentId: primary.id,
            name: primary.name,
            categories: sorted,
            uses: sorted.reduce((total, category) => total + category.uses, 0)
        });
    }

    // Groups follow their contents: the section you reach for most is the one
    // at the top. Ties fall back to the server's order, so an untouched install
    // still reads the way the server meant it to.
    groups.sort(
        (a, b) =>
            b.uses - a.uses ||
            (primaryById.get(a.parentId ?? '')?.displayOrder ?? 0) -
                (primaryById.get(b.parentId ?? '')?.displayOrder ?? 0) ||
            (a.name ?? '').localeCompare(b.name ?? '')
    );

    if (loose.length) {
        const sorted = [...loose].sort(byUsageThenOrder);

        // Always last. These are the leftovers, and promoting them above a real
        // section because one of them was tapped often would be confusing.
        groups.push({
            parentId: null,
            name: null,
            categories: sorted,
            uses: sorted.reduce((total, category) => total + category.uses, 0)
        });
    }

    return groups;
}
