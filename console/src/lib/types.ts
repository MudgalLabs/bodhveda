export interface Pagination {
    page: number;
    limit: number;
}

export interface PaginationMeta extends Pagination {
    total_items: number;
    total_pages: number;
}
