import { api as globalApi } from '../../../services/apiClient';

export const categoriesApi = {
    createCategory: globalApi.createCategory.bind(globalApi),
    listCategories: globalApi.listCategories.bind(globalApi),
    getCategory: globalApi.getCategory.bind(globalApi),
    updateCategory: globalApi.updateCategory.bind(globalApi),
    deleteCategory: globalApi.deleteCategory.bind(globalApi),
    getCategoryHierarchy: globalApi.getCategoryHierarchy.bind(globalApi),
    getCategoryInsights: globalApi.getCategoryInsights.bind(globalApi),
    getMemoriesInCategory: globalApi.getMemoriesInCategory.bind(globalApi),
    addMemoryToCategory: globalApi.addMemoryToCategory.bind(globalApi),
    removeMemoryFromCategory: globalApi.removeMemoryFromCategory.bind(globalApi)
};