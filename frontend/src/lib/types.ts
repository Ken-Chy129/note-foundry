export type Visibility = "private" | "public";

export interface KnowledgeSpace {
  id: string;
  name: string;
  visibility: Visibility;
}

export interface Directory {
  id: string;
  spaceId: string;
  parentId: string | null;
  name: string;
}

export interface PublishedContent {
  title: string;
  slug: string;
  markdown: string;
  publishedAt: string;
}

export interface LearningNote {
  id: string;
  spaceId: string;
  directoryId: string | null;
  title: string;
  slug: string;
  markdown: string;
  version: number;
  published: PublishedContent | null;
  updatedAt: string;
}

export interface LearningNoteSummary {
  id: string;
  spaceId: string;
  directoryId: string | null;
  title: string;
  slug: string;
  version: number;
  isPublished: boolean;
  updatedAt: string;
}

export type LearningSourceKind = "manual" | "url" | "pdf";
export type LearningSourceProcessingStatus = "pending" | "processing" | "ready" | "failed";

export interface LearningSourceSummary {
  id: string;
  kind: LearningSourceKind;
  spaceId: string | null;
  title: string;
  captureNote: string;
  originalUrl: string | null;
  processingStatus: LearningSourceProcessingStatus;
  createdAt: string;
  updatedAt: string;
}

export interface LearningSource extends LearningSourceSummary {
  content: string;
  normalizedUrl: string | null;
  failureMessage: string | null;
}

export interface PublishedNote {
  id: string;
  spaceId: string;
  directoryId: string | null;
  title: string;
  slug: string;
  markdown: string;
  publishedAt: string;
}

export interface Tag {
  id: string;
  name: string;
}

export interface LinkedNote {
  id: string;
  spaceId: string;
  title: string;
  slug: string;
}

export interface Attachment {
  id: string;
  noteId: string;
  originalName: string;
  mediaType: string;
  sizeBytes: number;
  sha256: string;
  publishedAt: string | null;
  createdAt: string;
}

export interface SearchResult {
  id: string;
  spaceId: string;
  title: string;
  slug: string;
  snippet: string;
  rank: number;
  updatedAt: string;
}

export interface Pagination {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
}

export interface PageResponse<T> {
  data: T[];
  pagination: Pagination;
}

export interface DataResponse<T> {
  data: T[];
}

export interface SessionResponse {
  owner: {
    githubUserId: number;
    login: string;
    avatarUrl: string;
  };
  expiresAt: string;
}

export interface APIErrorBody {
  error: {
    code: string;
    message: string;
  };
}
