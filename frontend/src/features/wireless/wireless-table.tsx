import type { ColumnDef } from '@tanstack/react-table';
import type { ReactNode } from 'react';

import { useMemo } from 'react';

import { DataTable, DataTableColumnHeader } from '@/components/ui/data-table';
import { usePageStorageKeys } from '@/hooks/use-page-storage-keys';
import { useTableState } from '@/hooks/use-table-state';

interface Props<T extends { id: string }> {
    actions?: (row: T) => ReactNode;
    data: T[];
    fields: [keyof T & string, string][];
    label: string;
    onRowClick?: (row: T) => void;
    tableKey: string;
}

export function WirelessTable<T extends { id: string }>({
    actions,
    data,
    fields,
    label,
    onRowClick,
    tableKey,
}: Props<T>) {
    const state = useTableState({ filterParamName: `${tableKey}Q`, pageParamName: `${tableKey}Page` });
    const { table } = usePageStorageKeys();
    const columns = useMemo<ColumnDef<T>[]>(() => {
        const result: ColumnDef<T>[] = fields.map(([key, title]) => ({
            accessorKey: key,
            cell: ({ row }) => {
                const value = row.original[key];

                return (
                    <span className="break-all">
                        {typeof value === 'boolean'
                            ? value
                                ? 'In scope'
                                : 'Outside scope'
                            : Array.isArray(value)
                              ? value.join(', ')
                              : String(value ?? '')}
                    </span>
                );
            },
            header: ({ column }) => (
                <DataTableColumnHeader
                    column={column}
                    title={title}
                />
            ),
            meta: { searchable: true },
        }));

        if (actions) {
            result.push({
                cell: ({ row }) => <div onClick={(event) => event.stopPropagation()}>{actions(row.original)}</div>,
                header: 'Actions',
                id: 'actions',
            });
        }

        return result;
    }, [actions, fields]);

    return (
        <DataTable
            columns={columns}
            data={data}
            empty={{ entityName: label.toLowerCase() }}
            filterValue={state.filter}
            getRowId={(row) => row.id}
            label={label}
            onFilterChange={state.setFilter}
            onPageChange={state.setPage}
            onRowClick={onRowClick}
            pageIndex={state.pageIndex}
            storageKey={`${table}:${tableKey}`}
        />
    );
}
