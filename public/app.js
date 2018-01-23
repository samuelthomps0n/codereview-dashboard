fetch('http://localhost:8081/webhook', {
	method: 'get'
});

new Vue({
	el: '#app',

	data: {
		ws: null, // Our websocket
		mergeRequests: '', // A running list of chat messages displayed on the screen
		sortOrder: 'desc',
		filterRepo: '',
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
			
			var user = [...self.users.Users].filter(function(user) {
				return user.id === id;
			})
			if(Object.keys(user).length !== 0) {
				return user[0]['avatar_url'];
			}
			return '';
		},
		filteredRepo: function (mrs) {
			var self = this;
			
			return mrs.filter(function(mr) {
				if(self.filterRepo !== '') {
					return mr.MergeRequest.project_id === self.filterRepo;
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

		fetch("/api/labels")
			.then(r => r.json())
			.then(json => {
				this.labels = json;
			});
		fetch("/api/users")
			.then(r => r.json())
			.then(json => {
				this.users = json;
			});
		fetch("/api/projects")
			.then(r => r.json())
			.then(json => {
				this.projects = json;
			});


		this.ws = new WebSocket('ws://' + window.location.host + '/ws');

		this.ws.addEventListener('message', function(e) {
			var mrd = JSON.parse(e.data);

			self.mergeRequests = mrd;

		});
	}
});