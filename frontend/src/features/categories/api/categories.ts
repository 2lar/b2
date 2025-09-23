import { api as globalApi } from '../../../services';

export const categoriesApi = {
    createCategory: globalApi.createCategory.bind(globalApi),
    listCategories: globalApi.listCategories.bind(globalApi),
    getCategory: globalApi.getCategory.bind(globalApi),
    updateCategory: globalApi.updateCategory.bind(globalApi),
    deleteCategory: globalApi.deleteCategory.bind(globalApi),
    getCategoryHierarchy: globalApi.getCategoryHierarchy?.bind(globalApi),
    getCategoryInsights: globalApi.getCategoryInsights?.bind(globalApi),
    getNodesInCategory: globalApi.getNodesInCategory?.bind(globalApi),
    assignNodeToCategory: globalApi.assignNodeToCategory?.bind(globalApi),
    removeNodeFromCategory: globalApi.removeNodeFromCategory?.bind(globalApi)
};
