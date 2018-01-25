// Force the Websocket to update
fetch('http://' + window.location.host + '/webhook', {
	method: 'get'
});

new Vue({
	el: '#app',

	data: {
		ws: null,
		mergeRequests: '',
		sortOrder: 'desc',
		activeProject: '',
		filterTeam: '',
		labels: {},
		users: {},
		projects: {},
		filterLabels: []
	},

	computed: {
		orderedMRs: function () {
			var self = this;
			
			var mrs = self.mergeRequests.MergeRequests;
			return _.orderBy(mrs, function(mr) {
				return mr.MergeRequest.created_at;
			}, [self.sortOrder])
		}
	},

	methods: {
		userAvatar: function(id) {
			var self = this;
			
			var user = [...self.users].filter(function(user) {
				return user.id === id;
			})
			if(Object.keys(user).length !== 0) {
				return user[0]['avatar_url'];
			}
			return '';
		},
		activeLabels: function(id) {
			var self = this;

			if(Object.keys(self.labels).length !== 0) {
				return [...self.labels].filter(function(label) {
					return label.project_id === self.activeProject;
				})
			}
		},
		filteredRepo: function (mrs) {
			var self = this;
			
			return mrs.filter(function(mr) {
				if(self.activeProject !== '') {
					return mr.MergeRequest.project_id === self.activeProject;
				}
				return true;
			})
		},
		filteredLabel: function (mrs) {
			var self = this;

			return mrs.filter(function(mr) {
				if(Object.keys(self.filterLabels).length !== 0) {
					return self.filterLabels.some(function(value) {
						return mr.MergeRequest.labels.includes(value);
					})
				}
				return true;
			})
		}
	},

	created: function() {
		var self = this;

		fetch("/api/users")
			.then(r => r.json())
			.then(json => {
				self.users = json;
			});
		fetch("/api/projects")
			.then(r => r.json())
			.then(json => {
				self.projects = json.Project;

				self.labels = _.map(self.projects, function(value, key) {
					return { labels: value.Labels, project_id: value.project_id };
				});

			});

		self.ws = new WebSocket('ws://' + window.location.host + '/ws');

		self.ws.addEventListener('message', function(e) {
			var mrd = JSON.parse(e.data);

			self.mergeRequests = mrd;

		});
	},

	watch: {
		activeProject: function(val, oldVal) {
			this.filterLabels = [];
		}
	}

});