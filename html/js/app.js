var searchApp = angular.module('searchApp', ['ngRoute', 'searchControllers']);

searchApp.config(['$routeProvider',
  function($routeProvider) {
    $routeProvider.
      when('/search', {
        templateUrl: '/tmpl/search.html',
        controller: 'SearchCtrl'
      }).
      otherwise({
        redirectTo: '/search'
      });
  }]);
